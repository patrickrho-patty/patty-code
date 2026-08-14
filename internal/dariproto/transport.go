package dariproto

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// TransportConfig configures the DARI transport.
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

// TransportConn wraps a TLS connection with DARI framing.
type TransportConn struct {
	conn   net.Conn
	config TransportConfig
	mu     sync.Mutex
}

// DialTCP dials a DARI peer over TLS/TCP.
func DialTCP(addr string, tlsConfig *tls.Config, config TransportConfig) (*TransportConn, error) {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         ALPNProtocols(),
			MinVersion:         tls.VersionTLS13,
		}
	}
	if tlsConfig.NextProtos == nil {
		tlsConfig.NextProtos = ALPNProtocols()
	}

	dialer := &net.Dialer{Timeout: 30 * time.Second}
	rawConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dari: dial %s: %w", addr, err)
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		rawConn.Close()
		return nil, fmt.Errorf("dari: TLS handshake: %w", err)
	}

	tc := &TransportConn{
		conn:   tlsConn,
		config: config,
	}

	if err := tc.sendPreface(); err != nil {
		tc.Close()
		return nil, fmt.Errorf("dari: send preface: %w", err)
	}

	return tc, nil
}

// sendPreface writes the DARI connection preface.
func (tc *TransportConn) sendPreface() error {
	tc.conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	_, err := tc.conn.Write(LegacyPaper1Preface)
	return err
}

// recvPreface reads and validates the DARI connection preface.
func (tc *TransportConn) recvPreface() error {
	tc.conn.SetReadDeadline(time.Now().Add(tc.config.ReadTimeout))
	preface := make([]byte, len(LegacyPaper1Preface))
	if _, err := io.ReadFull(tc.conn, preface); err != nil {
		return err
	}
	for i, b := range LegacyPaper1Preface {
		if preface[i] != b {
			return fmt.Errorf("dari: invalid preface byte %d", i)
		}
	}
	return nil
}

// SendRecord writes a DARI record.
func (tc *TransportConn) SendRecord(rec *Record) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.conn.SetWriteDeadline(time.Now().Add(tc.config.WriteTimeout))
	return EncodeRecord(tc.conn, rec)
}

// RecvRecord reads a DARI record.
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

// RecvAuthChallenge receives and decodes an AUTH_CHALLENGE control message.
func (tc *TransportConn) RecvAuthChallenge() (*AuthChallengeMessage, error) {
	record, err := tc.RecvRecord()
	if err != nil {
		return nil, err
	}
	if record.Kind != KindControl || MessageType(record.MessageType) != MsgAuthChallenge {
		return nil, fmt.Errorf("dari: expected AUTH_CHALLENGE, got kind=%d msg=%d", record.Kind, record.MessageType)
	}
	var challenge AuthChallengeMessage
	if err := UnmarshalCBOR(record.Payload, &challenge); err != nil {
		return nil, fmt.Errorf("dari: decode AUTH_CHALLENGE: %w", err)
	}
	return &challenge, nil
}

// AuthProof encodes and sends an AUTH_PROOF control message.
func (tc *TransportConn) AuthProof(proof *AuthProofMessage) error {
	payload, err := MarshalCBOR(proof)
	if err != nil {
		return fmt.Errorf("dari: encode AUTH_PROOF: %w", err)
	}
	return tc.SendControl(MsgAuthProof, nil, payload)
}

// Close closes the underlying connection.
func (tc *TransportConn) Close() error {
	return tc.conn.Close()
}

// Ensure binary import used
var _ = binary.BigEndian
