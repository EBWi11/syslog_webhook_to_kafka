package common

import (
	"fmt"
	"github.com/bytedance/sonic"
	"time"

	"github.com/vjeantet/grok"
	"gopkg.in/mcuadros/go-syslog.v2"
)

// SyslogMessage represents a parsed syslog message
type SyslogMessage struct {
	Format    string                 `json:"format"`
	Timestamp time.Time              `json:"timestamp"`
	Hostname  string                 `json:"hostname"`
	AppName   string                 `json:"app_name"`
	ProcID    string                 `json:"proc_id"`
	MsgID     string                 `json:"msg_id"`
	Priority  int                    `json:"priority"`
	Severity  int                    `json:"severity"`
	Facility  int                    `json:"facility"`
	Content   string                 `json:"content"`
	Raw       map[string]interface{} `json:"raw,omitempty"`
}

type SyslogConfig struct {
	listen   string
	protocol string
	format   string
	msgChan  chan []byte

	innerChannel syslog.LogPartsChannel
	msgHandler   *syslog.ChannelHandler
	server       *syslog.Server

	grok        *grok.Grok
	grokPattern string
}

func NewSyslog(listen, protocol string, format string, msgChan chan []byte) (*SyslogConfig, error) {
	var err error

	s := &SyslogConfig{
		listen:   listen,
		protocol: protocol,
		format:   format,
		msgChan:  msgChan,
	}

	s.innerChannel = make(syslog.LogPartsChannel)
	s.msgHandler = syslog.NewChannelHandler(s.innerChannel)

	//s.grok, _ = grok.NewWithConfig(&grok.Config{
	//	NamedCapturesOnly:   named_captures_only,
	//	SkipDefaultPatterns: skip_default_patterns,
	//	RemoveEmptyValues:   remove_empty_values,
	//})
	//s.grokPattern = grokPattern

	s.server = syslog.NewServer()

	// Set format based on config
	switch s.format {
	case "":
		s.server.SetFormat(syslog.RFC5424)
	case "RFC3164":
		s.server.SetFormat(syslog.RFC3164)
	case "RFC5424":
		s.server.SetFormat(syslog.RFC5424)
	case "RFC6587":
		s.server.SetFormat(syslog.RFC6587)
	default:
		return nil, fmt.Errorf("unsupported syslog format: %s", s.format)
	}

	s.server.SetHandler(s.msgHandler)

	switch s.protocol {
	case "tcp":
		err = s.server.ListenTCP(s.listen)
	case "udp":
		err = s.server.ListenUDP(s.listen)
	case "unixgram":
		err = s.server.ListenUnixgram(s.listen)
	default:
		return nil, fmt.Errorf("unsupported syslog protocol: %s", s.protocol)
	}

	if err != nil {
		return nil, fmt.Errorf("server listen error: %s", err.Error())
	}

	err = s.server.Boot()
	if err != nil {
		return nil, fmt.Errorf("server boot error: %s", err.Error())
	}

	return s, nil
}

func (s *SyslogConfig) Run() {
	go func(channel syslog.LogPartsChannel) {
		var data = make(map[string]interface{})
		for logParts := range channel {
			for k, v := range logParts {
				data[k] = v
			}
			dataBytes, err := sonic.Marshal(data)
			if err != nil {
				fmt.Println("syslog marshal json err: ", err.Error())
				continue
			}
			s.msgChan <- dataBytes
		}
	}(s.innerChannel)
}

func (s *SyslogConfig) Stop() {
	_ = s.server.Kill()
}

func (s *SyslogConfig) ListenAddr() string {
	return s.listen
}
