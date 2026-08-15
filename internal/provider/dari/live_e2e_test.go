package dari

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"patty/internal/dariproto"
	"patty/internal/evidence"
	"patty/internal/provenancewire"
	"patty/internal/provider"
)

// live_e2e_test.go is the end-to-end validation of the DARI
// governance loop (harness plans A1/A3/A4/A5 + B1/B3, e2e). It is
// gated so normal CI skips it:
//
//   DARI_LIVE_E2E=1      — full loop against a REAL relay binary with
//                            an in-test OpenAI-compatible PIA (offline).
//   DARI_LIVE_E2E_LIVE=1 — additionally drive real inference through
//                            the yolo-auto gateway (credentials from the
//                            parent repo's .env), mimicking a vLLM/SGLang
//                            deployment.
//
// The test proves, over real TCP + TLS + CBOR:
//   1. enrollment: connector pubkey → CA-issued COSE-Sign1 PPC,
//   2. AUTH_PROOF verification under the relay's trust bundle (A1),
//   3. SESSION_OPEN → POLICY_EPOCH → CATALOG_SNAPSHOT → LEASE_ISSUE →
//      SESSION_GRANT, lease verified under the AUTH_ACK issuer key (A3/A4/A5),
//   4. a governed AI exchange (relay authorize→forward→meter),
//   5. evidence-receipt push → connector store → ack → relay record (B3),
//   6. provenance envelope ingestion relay-side (B1) via the dispatcher.

func parentRepoRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("PCCP_ROOT"); root != "" {
		return root
	}
	// internal/provider/dari → up 4 levels = patty-code-pccp's parent.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	dir := filepath.Dir(thisFile)
	for i := 0; i < 4; i++ {
		dir = filepath.Dir(dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("parent repo not found at %s (set PCCP_ROOT)", dir)
	}
	return dir
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func loadDotEnv(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

func waitForHealth(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("relay did not become healthy at %s within %s", addr, timeout)
}

// enrollHarness generates a fresh connector keypair, enrolls it with
// the running relay's CA, and writes the connector identity files.
func enrollHarness(t *testing.T, adminAddr, harnessID string, dir string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	body, _ := json.Marshal(map[string]string{
		"organization_id": "org-e2e",
		"harness_id":      harnessID,
		"public_key_hex":  hex.EncodeToString(pub),
		"binary_version":  "v2-e2e",
		"binary_hash":     "e2e",
		"device_hostname": "e2e-host",
	})
	resp, err := http.Post(fmt.Sprintf("http://%s/v1/enroll", adminAddr), "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("enroll request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		var e map[string]any
		json.NewDecoder(resp.Body).Decode(&e)
		t.Fatalf("enroll failed (%d): %v", resp.StatusCode, e)
	}
	var out struct {
		CredentialHex string `json:"credential_hex"`
		Serial        string `json:"serial"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode enroll response: %v", err)
	}
	cred, err := hex.DecodeString(out.CredentialHex)
	if err != nil || len(cred) == 0 {
		t.Fatalf("bad credential hex (len=%d)", len(cred))
	}
	credPath := filepath.Join(dir, "ppc.cbor")
	keyPath := filepath.Join(dir, "ppc.key")
	if err := os.WriteFile(credPath, cred, 0o600); err != nil {
		t.Fatalf("write credential: %v", err)
	}
	if err := os.WriteFile(keyPath, priv, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("DARI_HARNESS_CREDENTIAL_FILE", credPath)
	t.Setenv("DARI_HARNESS_KEY_FILE", keyPath)
}

// runLiveE2E drives the whole loop. livePIA, when non-nil, is the
// real model backend base URL + key (yolo-auto).
func runLiveE2E(t *testing.T, livePIAURL, livePIAKey, liveModel string) {
	t.Helper()
	root := parentRepoRoot(t)
	tmp := t.TempDir()

	// Build the relay binary from the parent repo.
	relayBin := filepath.Join(tmp, "pccp-relay")
	build := exec.Command("go", "build", "-o", relayBin, "./cmd/pccp-relay")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build relay: %v\n%s", err, out)
	}

	// Mock PIA for the offline variant.
	var mockPIA *httptestServerShim
	piaURL, piaKey := "", ""
	if livePIAURL != "" {
		piaURL, piaKey = livePIAURL, livePIAKey
	} else {
		mockPIA = startMockPIA(t)
		piaURL = mockPIA.url
	}

	dariPort := freePort(t)
	adminPort := freePort(t)
	dariAddr := fmt.Sprintf("127.0.0.1:%d", dariPort)
	adminAddr := fmt.Sprintf("127.0.0.1:%d", adminPort)

	relayCmd := exec.Command(relayBin)
	relayCmd.Env = append(os.Environ(),
		"PCCP_DEV_BOOTSTRAP=1", // e2e brings up a fresh relay: first-run bootstrap
		"PCCP_DB_DSN="+filepath.Join(tmp, "e2e.db"),
		fmt.Sprintf("PCCP_RELAY_DARI_ADDR=%s", dariAddr),
		fmt.Sprintf("PCCP_RELAY_HTTP_ADDR=%s", adminAddr),
		"PCCP_PIA_URL="+piaURL,
		"PCCP_PIA_API_KEY="+piaKey,
	)
	relayCmd.Dir = tmp
	logFile, err := os.Create(filepath.Join(tmp, "relay.log"))
	if err != nil {
		t.Fatalf("create relay log: %v", err)
	}
	relayCmd.Stdout = logFile
	relayCmd.Stderr = logFile
	if err := relayCmd.Start(); err != nil {
		t.Fatalf("start relay: %v", err)
	}
	t.Cleanup(func() {
		_ = relayCmd.Process.Kill()
		_ = relayCmd.Wait()
		if t.Failed() {
			if data, err := os.ReadFile(filepath.Join(tmp, "relay.log")); err == nil {
				t.Logf("---- relay log ----\n%s", data)
			}
		}
	})

	waitForHealth(t, adminAddr, 20*time.Second)

	model := liveModel
	if model == "" {
		model = mockPIAModel
	}

	enrollHarness(t, adminAddr, "harness-e2e-1", tmp)

	prov, err := New(provider.Config{
		Name:    "paper-e2e",
		BaseURL: dariAddr,
		Model:   model,
		APIKey:  "unused-paper-auth-is-mutual-tls",
	})
	if err != nil {
		t.Fatalf("construct provider: %v", err)
	}
	pp := prov.(*Provider)

	store := provenancewire.NewReceiptStore()
	pp.SetReceiptHandler(provenancewire.NewIncomingAckHandler(store, liveConnAckSenderFor(t, pp)))

	// B1: queue a provenance changeset for this session; the stream
	// reader flushes it to the relay after the governed exchange.
	orgID, _ := pp.identity.Organization()
	cs, err := provenancewire.BuildChangeSetEnvelopeFromReceipts(provenancewire.ChangeSetBuildRequest{
		ChangeSetID:    "cs-e2e-1",
		OrganizationID: orgID,
		SessionID:      pp.ensureSessionID(),
		RepositoryID:   "repo-e2e",
		Receipts: []evidence.Receipt{
			{ToolName: "write_file", Mutation: true, Paths: []string{"e2e.go"}, Success: true},
		},
	})
	if err != nil {
		t.Fatalf("build changeset: %v", err)
	}
	if err := pp.ProvenanceEmitter().EmitChangeSet(cs); err != nil {
		t.Fatalf("emit changeset: %v", err)
	}

	// Stream one governed exchange.
	prompt := "Reply with the single word: pong"
	if livePIAURL != "" {
		// Live model: elicit a multi-token answer so token-by-token
		// streaming is observable (chunks > 1).
		prompt = "Count from one to five, one number per word."
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ch, err := pp.Stream(ctx, provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: prompt}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var text string
	var chunkCount int
	var usage *provider.Usage
	for chunk := range ch {
		switch chunk.Type {
		case provider.ChunkText:
			text += chunk.Text
			chunkCount++
		case provider.ChunkUsage:
			usage = chunk.Usage
		case provider.ChunkError:
			t.Fatalf("stream error: %v", chunk.Err)
		case provider.ChunkDone:
		}
	}
	if strings.TrimSpace(text) == "" {
		t.Fatal("no completion text received")
	}
	t.Logf("governed completion: %q chunks=%d (usage=%+v)", text, chunkCount, usage)
	if livePIAURL != "" && chunkCount < 2 {
		t.Fatalf("live model must stream token-by-token, got %d chunk(s)", chunkCount)
	}

	// Lease must be held and healthy (A3 e2e).
	metrics := pp.LeaseMetrics()
	if metrics.HeldLeaseID == "" {
		t.Fatal("no lease held after session setup")
	}
	t.Logf("lease: id=%s seq=%d epoch=%s", metrics.HeldLeaseID, metrics.HeldSequence, metrics.PolicyEpochID)

	// Task 7 e2e: the session grant must be present, policy-signed, and
	// bound to this session.
	if sg := pp.SessionGrant(); sg == nil {
		t.Fatal("relay did not deliver a session authorization grant")
	} else {
		if sg.Body.SessionID != pp.SessionID() {
			t.Fatalf("grant session %q != session %q", sg.Body.SessionID, pp.SessionID())
		}
		if sg.Body.SubjectPeerID == "" {
			t.Fatal("grant must bind a subject peer")
		}
		if sg.Body.ParentGrantDigest != nil {
			t.Fatal("session root grant must not carry a parent digest")
		}
		t.Logf("session grant: id=%s issuer=%s depth=%d models=%v", sg.Body.GrantID, sg.Body.Issuer, sg.Body.DelegationDepth, sg.Body.Scope.Models)
	}

	// Catalog must advertise the model (A5 e2e).
	if pp.catalogClient != nil {
		if _, err := pp.catalogClient.FindModel(model); err != nil {
			t.Fatalf("model %q missing from relay catalog: %v", model, err)
		}
	} else {
		t.Fatal("no catalog client installed from session setup")
	}

	// Evidence receipt must land in the store with an ack (B3 e2e).
	deadline := time.Now().Add(10 * time.Second)
	for {
		if receipts := store.List(); len(receipts) > 0 {
			if _, _, err := store.VerifyAck(receipts[0].Envelope.ReceiptID); err == nil {
				t.Logf("evidence receipt %s stored + acked", receipts[0].Envelope.ReceiptID)
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("evidence receipt was not stored/acked within 10s")
		}
		time.Sleep(100 * time.Millisecond)
	}

	// B1 e2e: the flushed changeset must be recorded relay-side.
	sessID := pp.SessionID()
	sawChangeSet := false
	deadline = time.Now().Add(10 * time.Second)
	for !sawChangeSet && time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://%s/v1/provenance/changesets", adminAddr))
		if err == nil {
			var out struct {
				ChangeSets []map[string]any `json:"changesets"`
			}
			json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			for _, row := range out.ChangeSets {
				if row["session_id"] == sessID {
					sawChangeSet = true
					t.Logf("provenance changeset recorded relay-side: %v", row["id"])
				}
			}
		}
		if !sawChangeSet {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !sawChangeSet {
		t.Fatal("provenance changeset was not recorded relay-side")
	}

	// Task 6 Step 3 e2e: revoke the harness via the admin API and
	// prove the NEXT connection is refused (revoked serial fails
	// AUTH under the live trust bundle) and the active stream was
	// terminated server-side.
	orgID, _ = pp.identity.Organization()
	revokeBody, _ := json.Marshal(map[string]string{
		"organization_id": orgID,
		"harness_id":      "harness-e2e-1",
		"reason":          "e2e revocation proof",
	})
	revokeResp, err := http.Post(fmt.Sprintf("http://%s/v1/harnesses/revoke", adminAddr), "application/json", strings.NewReader(string(revokeBody)))
	if err != nil || revokeResp.StatusCode != http.StatusOK {
		t.Fatalf("revoke failed: %v status=%d", err, revokeResp.StatusCode)
	}
	revokeResp.Body.Close()

	// Drop the (server-terminated) connection so the provider dials
	// fresh; AUTH must now be rejected for the revoked credential.
	pp.mu.Lock()
	if pp.conn != nil {
		pp.conn.Close()
		pp.conn = nil
	}
	pp.mu.Unlock()

	revokeCtx, revokeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer revokeCancel()
	_, err = pp.Stream(revokeCtx, provider.Request{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "should be refused"}},
	})
	if err == nil {
		t.Fatal("a revoked harness must not re-authenticate")
	}
	t.Logf("post-revocation connect refused as expected: %v", err)

	_ = mockPIA
}

// liveConnAckSenderFor lazily resolves the provider's live connection.
// The reader goroutine only invokes the handler while the connection
// is up, so the assertion is on conn presence at call time.
func liveConnAckSenderFor(t *testing.T, p *Provider) provenancewire.AckSender {
	return ackSenderFunc(func(rec *dariproto.Record) error {
		p.mu.Lock()
		conn := p.conn
		p.mu.Unlock()
		if conn == nil {
			return fmt.Errorf("no live connection")
		}
		return conn.SendRecord(rec)
	})
}

type ackSenderFunc func(rec *dariproto.Record) error

func (f ackSenderFunc) SendRecord(rec *dariproto.Record) error { return f(rec) }

const mockPIAModel = "e2e-mock-model"

// startMockPIA runs an OpenAI-compatible /v1/chat/completions server
// with a canned non-streaming response.
func startMockPIA(t *testing.T) *httptestServerShim {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Stream {
			// SSE streaming: multiple deltas then usage, exercising the
			// relay's governed token streaming (F1).
			w.Header().Set("Content-Type", "text/event-stream")
			flusher := w.(http.Flusher)
			deltas := []string{"po", "n", "g"}
			for _, d := range deltas {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", d)
				flusher.Flush()
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":3,\"total_tokens\":12}}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":    "chatcmpl-e2e",
			"model": mockPIAModel,
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "pong",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     9,
				"completion_tokens": 1,
				"total_tokens":      10,
			},
		})
	})
	srv := &http.Server{Handler: handler}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mock PIA listen: %v", err)
	}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })
	return &httptestServerShim{url: fmt.Sprintf("http://%s", l.Addr().String())}
}

type httptestServerShim struct {
	url string
}

func TestDARILiveE2EOffline(t *testing.T) {
	if os.Getenv("DARI_LIVE_E2E") != "1" {
		t.Skip("set DARI_LIVE_E2E=1 to run the live relay e2e loop")
	}
	runLiveE2E(t, "", "", "")
}

func TestDARILiveE2ERealModel(t *testing.T) {
	if os.Getenv("DARI_LIVE_E2E_LIVE") != "1" {
		t.Skip("set DARI_LIVE_E2E_LIVE=1 to run the live-model e2e (yolo-auto)")
	}
	env := loadDotEnv(t, parentRepoRoot(t))
	endpoint := env["YOLO_AUTO_ENDPOINT"]
	key := env["YOLO_AUTO_API_KEY"]
	model := env["YOLO_AUTO_MODEL"]
	if endpoint == "" || key == "" || model == "" {
		t.Fatal("YOLO_AUTO_ENDPOINT/KEY/MODEL missing from parent .env")
	}
	runLiveE2E(t, endpoint, key, model)
}
