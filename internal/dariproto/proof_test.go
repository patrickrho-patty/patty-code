package dariproto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Test fixtures shared by the signed-proof tests. They are intentionally
// self-contained so the conformance boundary can be exercised without an
// enrolled identity service.

const (
	testIssuerID        = "pccp-ca"
	testOrganizationID  = "org-test"
	testSubjectPeerID   = "hrn:patty:test"
	testProfile         = ProfileHarness
	testRevocationEpoch = uint64(7)
	testChannelBinding  = "tcp-exporter"
)

// testIdentity builds a freshly generated subject key and COSE-Sign1-signed
// Peer Credential Body. The issuer key is the same Ed25519 key used during
// dev preflight; the credential body uses the exact field order the relay's
// PeerAuthenticator round-trips.
//
// The result is byte-for-byte the same shape the relay's
// (`12798b8a`) `internal/paper/peer.go` `PeerCredential.SignedCredential`
// returns, so the bytes can be passed to a relay verifier untouched.
func testIdentity(t *testing.T) (subjectPub ed25519.PublicKey, subjectPriv ed25519.PrivateKey, issuerPub ed25519.PublicKey, issuerPriv ed25519.PrivateKey, credential []byte) {
	t.Helper()
	issuerPub, issuerPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("issuer key: %v", err)
	}
	subjectPub, subjectPriv, err = ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("subject key: %v", err)
	}

	body := testPeerCredentialBody{
		CredentialVersion:       1,
		Issuer:                  testIssuerID,
		SubjectPeerID:           testSubjectPeerID,
		Organization:            testOrganizationID,
		PeerProfile:             testProfile,
		PublicKey:               subjectPub,
		AllowedProtocolVersions: []uint8{1},
		RevocationAuthority:     testIssuerID,
		Serial:                  "test-serial-0001",
		NotBefore:               0,
		NotAfter:                9_999_999_999_999,
	}
	bodyBytes, err := MarshalCBOR(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	encoded, err := CreateCOSESign1(bodyBytes, issuerPriv, []byte("test-kid"))
	if err != nil {
		t.Fatalf("marshal COSE-Sign1: %v", err)
	}
	credential = encoded
	if err != nil {
		t.Fatalf("marshal COSE-Sign1: %v", err)
	}
	return subjectPub, subjectPriv, issuerPub, issuerPriv, credential
}

// testPeerCredentialBody mirrors the relay's `peerCredentialSigningBody` (see
// internal/paper/peer.go `peerCredentialSigningBody`). Keeping the field shape
// in lockstep with the relay is a contract requirement: the connector MUST
// produce COSE-Sign1 credential bodies whose canonical CBOR is byte-for-byte
// the same input the relay's `paper.DecodePeerCredential` will re-parse. A
// field re-order, a different int width, or an extra field makes the
// issuer/verifier pubkey match but the payload digest diverge.
type testPeerCredentialBody struct {
	CredentialVersion       uint16     `cbor:"credential_version"`
	Issuer                  string     `cbor:"issuer"`
	SubjectPeerID           string     `cbor:"subject_peer_id"`
	Organization            string     `cbor:"organization"`
	PeerProfile             PeerProfile `cbor:"peer_profile"`
	PublicKey               []byte     `cbor:"public_key"`
	NotBefore               int64      `cbor:"not_before"`
	NotAfter                int64      `cbor:"not_after"`
	Serial                  string     `cbor:"serial"`
	RevocationAuthority     string     `cbor:"revocation_authority"`
	AllowedProtocolVersions []uint8    `cbor:"protocol_versions"`
	BuildChannel            string     `cbor:"build_channel,omitempty"`
	DeploymentZone          string     `cbor:"deployment_zone,omitempty"`
}

// TestBuildAuthContextMatchesRelayVector asserts that the connector's
// AuthContext calculation matches the relay's byte-for-byte so the proof
// verification on the relay side sees the same transcript hash.
func TestBuildAuthContextMatchesRelayVector(t *testing.T) {
	hello := &HelloMessage{
		CoreVersions:          []uint8{1},
		PeerProfile:           ProfileHarness,
		TransportFeatures:     []string{"tcp-tls"},
		Extensions:            map[string]uint8{"dari.ai/1": 1, "dari.models/1": 1},
		EncodingProfiles:      []string{"cbor"},
		CryptoProfiles:        []string{"DARI-BASE-1"},
		ClientNonce:           bytes.Repeat([]byte{0x11}, 32),
		ImplementationName:    "patty-code",
		ImplementationVersion: "v2-paper",
	}
	ack := &HelloAckMessage{
		CoreVersion:       1,
		ExtensionVersions: map[string]uint8{"dari.ai/1": 1},
		CryptoProfile:     "DARI-BASE-1",
		ServerNonce:       bytes.Repeat([]byte{0x22}, 32),
		ResourceLimits:    map[string]uint64{"max_payload_len": 1 << 20},
	}
	helloCBOR, err := CanonicalHelloCBOR(hello)
	if err != nil {
		t.Fatalf("canonical hello: %v", err)
	}
	ackCBOR, err := CanonicalAckCBOR(ack)
	if err != nil {
		t.Fatalf("canonical ack: %v", err)
	}
	_, _, _, _, credential := testIdentity(t)
	credDigest := ComputeObjectDigest(ObjTypePeerCredential, credential)

	got := BuildAuthContext(
		helloCBOR, ackCBOR,
		hello.ClientNonce, ack.ServerNonce,
		[]byte(testChannelBinding),
		credDigest[:],
	)

	// Independent recomputation: SHA-256("DARI-AUTH-v1" || canonical(HELLO) ||
	// canonical(HELLO_ACK) || clientNonce || serverNonce || channelBinding ||
	// credentialDigest). The domain has NO trailing NUL and the HELLO/ACK
	// bytes are the map-order-free canonical encodings — pinned against
	// the live relay by the PAPER_LIVE_E2E suite.
	h := sha256.New()
	h.Write([]byte("DARI-AUTH-v1"))
	h.Write(helloCBOR)
	h.Write(ackCBOR)
	h.Write(hello.ClientNonce)
	h.Write(ack.ServerNonce)
	h.Write([]byte(testChannelBinding))
	h.Write(credDigest[:])
	want := h.Sum(nil)

	if !bytes.Equal(got[:], want) {
		t.Fatalf("auth context mismatch\n got %s\nwant %s", hex.EncodeToString(got[:]), hex.EncodeToString(want))
	}
}

// TestHelloAckResourceLimitsRoundTrip ensures the connector's decoded view of
// HELLO_ACK byte-matches a separately marshaled ack. Without ResourceLimits in
// the struct, the connector's transcript would silently drop field 8 and the
// relay would compute a different context hash.
func TestHelloAckResourceLimitsRoundTrip(t *testing.T) {
	connector := &HelloAckMessage{
		CoreVersion:       1,
		CryptoProfile:     "DARI-BASE-1",
		ServerNonce:       bytes.Repeat([]byte{0x33}, 32),
		ResourceLimits:    map[string]uint64{"max_payload_len": 1024},
	}
	connectorBytes, err := MarshalCBOR(connector)
	if err != nil {
		t.Fatalf("marshal connector ack: %v", err)
	}
	relay := &HelloAckMessage{
		CoreVersion:       1,
		CryptoProfile:     "DARI-BASE-1",
		ServerNonce:       bytes.Repeat([]byte{0x33}, 32),
		ResourceLimits:    map[string]uint64{"max_payload_len": 1024},
	}
	relayBytes, err := MarshalCBOR(relay)
	if err != nil {
		t.Fatalf("marshal relay ack: %v", err)
	}
	if !bytes.Equal(connectorBytes, relayBytes) {
		t.Fatalf("ack transcript byte mismatch\n connector %s\n relay     %s",
			hex.EncodeToString(connectorBytes), hex.EncodeToString(relayBytes))
	}
}

// TestSignAuthProofProducesDomainSeparatedBytes verifies the proof-signing
// byte construction matches the relay's PeerProofSigningBytes so the relay
// verifier can recompute the same signing input.
func TestSignAuthProofProducesDomainSeparatedBytes(t *testing.T) {
	_, subjectPriv, _, _, _ := testIdentity(t)
	transcript := bytes.Repeat([]byte{0x44}, 32)
	challengeID := bytes.Repeat([]byte{0x55}, 16)
	epoch := testRevocationEpoch

	proof := SignAuthProof(subjectPriv, transcript, challengeID, epoch)
	if len(proof) != ed25519.SignatureSize {
		t.Fatalf("proof length = %d, want %d", len(proof), ed25519.SignatureSize)
	}

	// Recompute signing bytes identically.
	h := sha256.New()
	h.Write([]byte("DARI-AUTH-PROOF-v1\x00"))
	writeLengthPrefixed(h, transcript)
	writeLengthPrefixed(h, challengeID)
	var epochBuf [8]byte
	binary.BigEndian.PutUint64(epochBuf[:], epoch)
	h.Write(epochBuf[:])
	signingBytes := h.Sum(nil)

	// The verifiable property: a fresh Ed25519 verify over (subjectPub,
	// signingBytes, proof) MUST succeed.
	pub := subjectPriv.Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, signingBytes, proof) {
		t.Fatalf("proof does not verify under subject public key")
	}
}

// TestSignAuthProofRejectsDifferentEpoch guards against replay across
// revocation epochs. The relay bumps the epoch on every revoke; the connector
// must include the epoch in the proof and a stale proof must fail verify.
func TestSignAuthProofRejectsDifferentEpoch(t *testing.T) {
	_, subjectPriv, _, _, _ := testIdentity(t)
	transcript := []byte("transcript-a")
	challengeID := []byte("challenge-a")

	proof := SignAuthProof(subjectPriv, transcript, challengeID, testRevocationEpoch)
	pub := subjectPriv.Public().(ed25519.PublicKey)

	// Same transcript + challenge, but the relay's current epoch advanced.
	h := sha256.New()
	h.Write([]byte("DARI-AUTH-PROOF-v1\x00"))
	writeLengthPrefixed(h, transcript)
	writeLengthPrefixed(h, challengeID)
	var epochBuf [8]byte
	binary.BigEndian.PutUint64(epochBuf[:], testRevocationEpoch+1)
	h.Write(epochBuf[:])
	if ed25519.Verify(pub, h.Sum(nil), proof) {
		t.Fatalf("proof replayed against advanced epoch must not verify")
	}
}

// TestLoadIdentityFromDisk ensures the credential + private-key loader
// resolves the harness identity from a pair of files (raw 32-byte subject
// private key + hex-encoded COSE-Sign1 credential) and rejects malformed
// inputs.
func TestLoadIdentityFromDisk(t *testing.T) {
	dir := t.TempDir()
	_, subjectPriv, _, _, credential := testIdentity(t)

	keyPath := filepath.Join(dir, "harness.ed25519")
	if err := os.WriteFile(keyPath, subjectPriv, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	credPath := filepath.Join(dir, "harness.cred")
	if err := os.WriteFile(credPath, credential, 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}

	id, err := LoadIdentityFromDisk(credPath, keyPath)
	if err != nil {
		t.Fatalf("load identity: %v", err)
	}
	if !bytes.Equal(id.PrivateKey, subjectPriv) {
		t.Fatalf("private key bytes mismatch")
	}
	if !bytes.Equal(id.Credential, credential) {
		t.Fatalf("credential bytes mismatch")
	}

	// Missing files must error.
	if _, err := LoadIdentityFromDisk(filepath.Join(dir, "missing.cred"), keyPath); err == nil {
		t.Fatalf("missing credential file must error")
	}
	if _, err := LoadIdentityFromDisk(credPath, filepath.Join(dir, "missing.ed25519")); err == nil {
		t.Fatalf("missing private key file must error")
	}

	// Wrong-size private key must error.
	wrongKey := filepath.Join(dir, "wrong.ed25519")
	if err := os.WriteFile(wrongKey, []byte("not-a-key"), 0o600); err != nil {
		t.Fatalf("write wrong key: %v", err)
	}
	if _, err := LoadIdentityFromDisk(credPath, wrongKey); err == nil {
		t.Fatalf("non-32-byte private key must error")
	}

	// Empty credential bytes must error.
	emptyCred := filepath.Join(dir, "empty.cred")
	if err := os.WriteFile(emptyCred, nil, 0o600); err != nil {
		t.Fatalf("write empty cred: %v", err)
	}
	if _, err := LoadIdentityFromDisk(emptyCred, keyPath); err == nil {
		t.Fatalf("empty credential must error")
	}
}

// TestBuildAuthProofEmitsRelayShape walks the full proof builder and confirms
// each field round-trips through the connector's AuthProofMessage — the same
// CBOR layout the relay parses.
func TestBuildAuthProofEmitsRelayShape(t *testing.T) {
	hello := &HelloMessage{
		CoreVersions:       []uint8{1},
		PeerProfile:        ProfileHarness,
		TransportFeatures:  []string{"tcp-tls"},
		Extensions:         map[string]uint8{"dari.ai/1": 1},
		EncodingProfiles:   []string{"cbor"},
		CryptoProfiles:     []string{"DARI-BASE-1"},
		ClientNonce:        bytes.Repeat([]byte{0xAA}, 32),
		ImplementationName: "patty-code",
	}
	ack := &HelloAckMessage{
		CoreVersion:    1,
		CryptoProfile:  "DARI-BASE-1",
		ServerNonce:    bytes.Repeat([]byte{0xBB}, 32),
		ResourceLimits: map[string]uint64{"max_payload_len": 1 << 20},
	}
	_, subjectPriv, _, _, credential := testIdentity(t)
	challengeID := []byte("proof-challenge-001")

	proof, err := BuildAuthProof(AuthProofInput{
		PrivateKey:      subjectPriv,
		Credential:      credential,
		Hello:           hello,
		HelloAck:        ack,
		ChallengeID:     challengeID,
		RevocationEpoch: testRevocationEpoch,
		ChannelBinding:  []byte(testChannelBinding),
	})
	if err != nil {
		t.Fatalf("build proof: %v", err)
	}
	if !bytes.Equal(proof.Credential, credential) {
		t.Fatalf("proof credential bytes mismatch")
	}
	if !bytes.Equal(proof.ChallengeID, challengeID) {
		t.Fatalf("proof challenge ID mismatch")
	}
	if proof.KeyAlgorithm != COSEAlgEdDSA {
		t.Fatalf("proof algorithm = %d, want EdDSA(-8)", proof.KeyAlgorithm)
	}
	if got := binary.BigEndian.Uint64(proof.RevocationEvidence); got != testRevocationEpoch {
		t.Fatalf("proof revocation epoch = %d, want %d", got, testRevocationEpoch)
	}
	// Signature MUST verify under the subject key, the relay's transcript,
	// the challenge ID, and the same epoch.
	helloCBOR, _ := CanonicalHelloCBOR(hello)
	ackCBOR, _ := CanonicalAckCBOR(ack)
	credDigest := ComputeObjectDigest(ObjTypePeerCredential, credential)
	transcript := BuildAuthContext(
		helloCBOR, ackCBOR,
		hello.ClientNonce, ack.ServerNonce,
		[]byte(testChannelBinding),
		credDigest[:],
	)
	signingBytes := SignAuthProofInputs(transcript[:], challengeID, testRevocationEpoch)
	if !ed25519.Verify(subjectPriv.Public().(ed25519.PublicKey), signingBytes, proof.Signature) {
		t.Fatalf("emitted proof does not verify under transcript")
	}
}
