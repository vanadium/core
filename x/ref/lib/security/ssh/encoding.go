// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssh

/*

func publicKeyFromCryptoKey(key crypto.PublicKey) (security.PublicKey, error) {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	case ed25519.PublicKey:
		return security.NewED25519PublicKey(k), nil
	case *rsa.PublicKey:
		return security.NewRSAPublicKey(k), nil
		}
	return nil, fmt.Errorf("unsupported key type %T", key)
}


// DecodePublicKeyPEM decodes the supplied PEM block to obtain a public key.
func DecodePublicKeyPEM(buf []byte) (security.PublicKey, error) {
	key, err := internal.LoadPEMPublicKey(bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	return publicKeyFromCryptoKey(key)
}

// DecodePublicKeySSH decodes the supplied ssh 'authorized hosts' format
// to obtain a public key.
func DecodePublicKeySSH(buf []byte) (security.PublicKey, error) {
	sshKey, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, err
	}
	key, err := internal.CryptoKeyFromSSHKey(sshKey)
	if err != nil {
		return nil, err
	}
	return publicKeyFromCryptoKey(key)
}

func parseAndMarshalOpenSSHKey(buf []byte) ([]byte, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey(buf)
	if err != nil {
		return nil, err
	}
	return pk.Marshal(), nil
}

// SSHSignatureMD5 returns the md5 signature for the supplied openssh
// "authorized hosts" format public key. It produces the same output
// as ssh-keygen -l -f <pub-key-file> -E md5
func SSHSignatureMD5(buf []byte) (string, error) {
	raw, err := parseAndMarshalOpenSSHKey(buf)
	if err != nil {
		return "", err
	}
	return md5Signature(raw), nil
}

// SSHSignatureSHA256 returns the base64 raw encoding sha256 signature for
// the supplied openssh "authorized hosts" format public key. It produces
// the same output as ssh-keygen -l -f <pub-key-file> -E sha256.
func SSHSignatureSHA256(buf []byte) (string, error) {
	raw, err := parseAndMarshalOpenSSHKey(buf)
	if err != nil {
		return "", err
	}
	return sha256Signature(raw), nil
}

func md5Signature(bytes []byte) string {
	const hextable = "0123456789abcdef"
	hash := md5.Sum(bytes)
	var repr [md5.Size * 3]byte
	for i, v := range hash {
		repr[i*3] = hextable[v>>4]
		repr[i*3+1] = hextable[v&0x0f]
		repr[i*3+2] = ':'
	}
	return string(repr[:len(repr)-1])
}

func sha256Signature(bytes []byte) string {
	hash := sha256.Sum256(bytes)
	return base64.RawStdEncoding.EncodeToString(hash[:])
}

// EncodePublicKeyBase64 encodes the supplied public key as a base64
// url encoded string. The underlying data format is DER.
func EncodePublicKeyBase64(key security.PublicKey) (string, error) {
	buf, err := key.MarshalBinary()
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buf), nil
}

// DecodePublicKeyBase64 decodes a public key from the supplied
// base64 url encoded string. It assumes that the underlying data format
// is DER.
func DecodePublicKeyBase64(key string) (security.PublicKey, error) {
	buf, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return security.UnmarshalPublicKey(buf)
}


*/
