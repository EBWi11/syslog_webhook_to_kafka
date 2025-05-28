package common

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaProducer struct {
	client   *kgo.Client
	topic    string
	keyField []string
	keyFlag  bool
}

func StringToList(checkKey string) []string {
	if len(checkKey) == 0 {
		return nil
	}
	var res []string
	var sb strings.Builder
	for i := 0; i < len(checkKey); i++ {
		if checkKey[i] == '\\' && i+1 < len(checkKey) && checkKey[i+1] == '.' {
			sb.WriteByte('.')
			i++
		} else if checkKey[i] == '.' {
			res = append(res, sb.String())
			sb.Reset()
		} else {
			sb.WriteByte(checkKey[i])
		}
	}
	if sb.Len() > 0 {
		res = append(res, sb.String())
	}
	return res
}

// UrlValueToMap converts url.Values (map[string][]string) to map[string]interface{}.
// Joins multiple values into a single string.
func UrlValueToMap(data map[string][]string) map[string]interface{} {
	res := make(map[string]interface{}, len(data))
	for k, v := range data {
		res[k] = strings.Join(v, "")
	}
	return res
}

// AnyToString converts various types to their string representation.
// Supports string, int, bool, float64, int64, and falls back to JSON for others.
func AnyToString(tmp interface{}) string {
	switch value := tmp.(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(value, 10)
	default:
		// Marshal to JSON string for unsupported types
		resBytes, _ := sonic.Marshal(tmp)
		return string(resBytes)
	}
}

// GetCheckData traverses a nested map[string]interface{} using a key path (checkKeyList).
// Returns the string value and whether it exists.
// Handles map, slice, JSON string, and URL query string as intermediate nodes.
func GetCheckData(data map[string]interface{}, checkKeyList []string) (res string, exist bool) {
	tmp := data
	res = ""
	keyListLen := len(checkKeyList) - 1
	for i, k := range checkKeyList {
		tmpRes, ok := tmp[k]
		if !ok || tmpRes == nil {
			// Key not found or value is nil
			return "", false
		}
		if i != keyListLen {
			switch value := tmpRes.(type) {
			case map[string]interface{}:
				// Continue traversing nested map
				tmp = value
			case []interface{}:
				// Convert slice to map with index keys
				tmpMapForList := make(map[string]interface{}, len(value))
				for idx, v := range value {
					tmpKey := "#_" + strconv.Itoa(idx)
					tmpMapForList[tmpKey] = v
				}
				tmp = tmpMapForList
			case string:
				// Try to parse as JSON if it looks like JSON
				if (strings.Contains(value, ":") || strings.Contains(value, "{") || strings.Contains(value, "[")) && len(value) > 2 {
					tmpValue := make(map[string]interface{})
					if err := sonic.Unmarshal([]byte(value), &tmpValue); err == nil {
						tmp = tmpValue
						continue
					}
				}
				// Try to parse as URL query string
				if tmpValue, err := url.ParseQuery(value); err == nil {
					tmp = UrlValueToMap(tmpValue)
					continue
				}
				// Not a traversable structure
				return "", false
			default:
				// Unsupported type for traversal
				return "", false
			}
		} else {
			// Last key, convert value to string
			res = AnyToString(tmpRes)
			exist = true
		}
	}
	if res == "" {
		return "", exist
	}
	return res, exist
}

func NewKafkaProducer(brokers []string, topic string, keyField []string) (*KafkaProducer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	kp := &KafkaProducer{
		client: client,
		topic:  topic,
	}

	if len(keyField) > 0 {
		kp.keyFlag = true
		kp.keyField = keyField
	}

	return kp, nil
}

func (p *KafkaProducer) SendMessage(msg []byte) error {
	var key []byte
	if p.keyFlag {
		// Parse JSON to get key field
		var data map[string]interface{}
		if err := sonic.Unmarshal(msg, &data); err != nil {
			return fmt.Errorf("failed to parse message for key: %w", err)
		}

		if keyStr, ok := GetCheckData(data, p.keyField); ok {
			key = []byte(keyStr)
		}
	}

	record := &kgo.Record{
		Topic: p.topic,
		Key:   key,
		Value: msg,
	}

	if err := p.client.ProduceSync(nil, record).FirstErr(); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	return nil
}

func (p *KafkaProducer) Close() error {
	p.client.Close()
	return nil
}
