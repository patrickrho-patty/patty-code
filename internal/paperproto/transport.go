package paperproto

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// TransportConfig configures the PAPER transport.
type TransportConfig struct {
	TLSConfig      *tls.Config
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	KeepAlive      time.Duration
	MaxMessageSize int
}

// DefaultTransportConfig returns sensible defaults.
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   30 * time.Second,
		KeepAlive:      30 * time.Second,
		MaxMessageSize: 2 * 1024 * 1024,
	}
}

// TransportConn wraps a TLS connection with PAPER framing.
type TransportConn struct {
	conn   net.Conn
	config TransportConfig
	mu     sync.Mutex
}

// DialTCP dials a PAPER peer over TLS/TCP.
func DialTCP(addr string, tlsConfig *tls.Config, config TransportConfig) (*TransportConn, error) {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{ALPNProtocol},
			MinVersion:         tls.VersionTLS13,
		}
	}
	if tlsConfig.NextProtos == nil {
		tlsConfig.NextProtos = []string{ALPNProtocol}
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	rawConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("paper: dial %s: %w", addr, err)
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("paper: TLS handshake: %w", err)
	}

	tc := &TransportConn{
		conn:   tlsConn,
		config: config,
	}

	if err := tc.sendPreface(); err != nil {
		tc.Close()
		return nil, fmt.Errorf("paper: send preface: %w", err)
	}

	return tc, nil
}

// sendPreface writes the PAPER connection preface.
func (tc *TransportConn) sendPreface() error {
	tc.conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	_, err := tc.conn.Write(PAPERPreface)
	return err
}

// recvPreface reads and validates the PAPER connection preface.
func (tc *TransportConn) recvPreface() error {
	tc.conn.SetReadDeadline(time.Now().Add(tc.config.ReadTimeout))
	preface := make([]byte, len(PAPERPreface))
	if _, err := io.ReadFull(tc.conn, preface); err != nil {
		return err
	}
	for i, b := range PAPERPreface {
		if preface[i] != b {
			return fmt.Errorf("paper: invalid preface byte %d", i)
		}
	}
	return nil
}

// SendRecord writes a PAPER record.
func (tc *TransportConn) SendRecord(rec *Record) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	return EncodeRecord(tc.conn, rec)
}

// RecvRecord reads a PAPER record.
func (tc *TransportConn) RecvRecord() (*Record, error) {
	tc.conn.SetReadDeadline(time.Now().Add(tc.config.ReadTimeout))
	return DecodeRecord(tc.conn)
}

// SendMessage sends a MESSAGE record.
func (tc *TransportConn) SendMessage(msgType MessageType, header, payload []byte, laneID, laneSeq uint64) error {
	return tc.SendRecord(&Record{
		Kind:         KindMessage,
		MessageType:  uint16(msgType),
		Header:       header,
		Payload:      payload,
		LaneID:       laneID,
		LaneSequence: laneSeq,
	})
}

// SendControl sends a CONTROL record.
func (tc *TransportConn) SendControl(msgType MessageType, header, payload []byte) error {
	return tc.SendRecord(&Record{
		Kind:        KindControl,
		MessageType: uint16(msgType),
		Header:      header,
		Payload:     payload,
	})
}

// Close closes the underlying connection.
func (tc *TransportConn) Close() error {
	return tc.conn.Close()
}

// Ensure binary import used
var _ = binary.BigEndian
