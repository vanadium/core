// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/security/serialization"
)

func encodeAndStore(obj interface{}, data, signature io.WriteCloser, signer serialization.Signer) error {
	if data == nil || signature == nil {
		return fmt.Errorf("encode: invalid data/signature handles data:%v sig:%v", data, signature)
	}
	swc, err := serialization.NewSigningWriteCloser(data, signature, signer, nil)
	if err != nil {
		return err
	}
	enc := vom.NewEncoder(swc)
	if err := enc.Encode(obj); err != nil {
		swc.Close()
		return err
	}
	return swc.Close()
}

func decodeFromStorage(obj interface{}, data, signature io.ReadCloser, publicKey security.PublicKey) error {
	if data == nil || signature == nil {
		return fmt.Errorf("decode: invalid data/signature handles data:%v sig:%v", data, signature)
	}
	defer data.Close()
	defer signature.Close()
	vr, err := serialization.NewVerifyingReader(data, signature, publicKey)
	if err != nil {
		return err
	}
	dec := vom.NewDecoder(vr)
	return dec.Decode(obj)
}

func saveLocked(ctx context.Context, state interface{}, writer CredentialsStoreReadWriter, signer serialization.Signer) error {
	wr, err := writer.RootsWriter(ctx)
	if err != nil {
		return err
	}
	data, signature, err := wr.Writers()
	if err != nil {
		return err
	}
	return encodeAndStore(state, data, signature, signer)
}

func handleRefresh(ctx context.Context, interval time.Duration, loader func() error) {
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-time.After(interval):
			case <-hupCh:
			case <-ctx.Done():
				return
			}
			if err := loader(); err != nil {
				vlog.Errorf("failed to reload principal: %v", err)
				return
			}
		}
	}()
}
