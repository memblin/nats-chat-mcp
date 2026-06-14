package nats

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestPurgeDirectThreadIntegration exercises PurgeDirectThread against a live
// JetStream server: it deletes a thread's messages in both directions and leaves
// other senders' DMs intact. Set NATS_TEST_URL (e.g. nats://127.0.0.1:14222) to
// run it; the test is skipped otherwise so the default suite needs no server.
func TestPurgeDirectThreadIntegration(t *testing.T) {
	url := os.Getenv("NATS_TEST_URL")
	if url == "" {
		t.Skip("set NATS_TEST_URL to run the purge integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	me := Identity{ID: "hme_purge", Name: "me"}
	alice := Identity{ID: "aalice_purge", Name: "alice"}
	carol := Identity{ID: "acarol_purge", Name: "carol"}

	sink := func(any) {}
	myClient, err := Connect(ctx, url, me, sink)
	if err != nil {
		t.Fatalf("connect me: %v", err)
	}
	defer myClient.Close(ctx)

	aliceClient, err := Connect(ctx, url, alice, sink)
	if err != nil {
		t.Fatalf("connect alice: %v", err)
	}
	defer aliceClient.Close(ctx)

	carolClient, err := Connect(ctx, url, carol, sink)
	if err != nil {
		t.Fatalf("connect carol: %v", err)
	}
	defer carolClient.Close(ctx)

	// Conversation: alice<->me both directions, plus an unrelated DM from carol.
	if _, err := aliceClient.SendDirect(ctx, me.ID, "hi from alice", ""); err != nil {
		t.Fatalf("alice->me: %v", err)
	}
	if _, err := aliceClient.SendDirect(ctx, me.ID, "second from alice", ""); err != nil {
		t.Fatalf("alice->me 2: %v", err)
	}
	if _, err := myClient.SendDirect(ctx, alice.ID, "reply from me", ""); err != nil {
		t.Fatalf("me->alice: %v", err)
	}
	if _, err := carolClient.SendDirect(ctx, me.ID, "hi from carol", ""); err != nil {
		t.Fatalf("carol->me: %v", err)
	}

	deleted, err := myClient.PurgeDirectThread(ctx, alice.ID)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("purge deleted %d messages, want 3 (2 inbound + 1 outbound)", deleted)
	}

	// My inbox should no longer hold any of alice's messages...
	aliceInMyInbox, err := myClient.directSeqsFrom(ctx, DirectSubject(me.ID), alice.ID)
	if err != nil {
		t.Fatalf("scan my inbox for alice: %v", err)
	}
	if len(aliceInMyInbox) != 0 {
		t.Errorf("my inbox still holds %d of alice's messages after purge, want 0", len(aliceInMyInbox))
	}
	// ...but carol's unrelated DM must survive.
	carolInMyInbox, err := myClient.directSeqsFrom(ctx, DirectSubject(me.ID), carol.ID)
	if err != nil {
		t.Fatalf("scan my inbox for carol: %v", err)
	}
	if len(carolInMyInbox) != 1 {
		t.Errorf("carol's DM count after purge = %d, want 1 (untouched)", len(carolInMyInbox))
	}

	// Outbound subject (alice's inbox) should be empty of my messages.
	aliceInboxFromMe, err := myClient.directSeqsFrom(ctx, DirectSubject(alice.ID), me.ID)
	if err != nil {
		t.Fatalf("scan alice inbox: %v", err)
	}
	if len(aliceInboxFromMe) != 0 {
		t.Errorf("alice inbox still holds %d of my messages after purge, want 0", len(aliceInboxFromMe))
	}

	// A second purge is a no-op (idempotent), deleting nothing.
	again, err := myClient.PurgeDirectThread(ctx, alice.ID)
	if err != nil {
		t.Fatalf("second purge: %v", err)
	}
	if again != 0 {
		t.Errorf("second purge deleted %d, want 0", again)
	}
}
