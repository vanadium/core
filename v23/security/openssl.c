// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

#include <openssl/crypto.h>
#include <openssl/evp.h>
#include <openssl/ec.h>
#include <openssl/ecdsa.h>
#include <openssl/rsa.h>
#include <openssl/err.h>
#include <openssl/x509.h>

// The functions below are to ensure that the call to ERR_get_error happens in
// the same thread as the call to the OpenSSL API.
// If the two were called from Go, the goroutine might be pre-empted and
// rescheduled on another thread leading to an inconsistent error.

// d2i_ECPrivateKey + ERR_get_error in a single function.
EC_KEY *openssl_d2i_ECPrivateKey(const unsigned char *data, long len, unsigned long *e)
{
	EC_KEY *k = d2i_ECPrivateKey(NULL, &data, len);
	if (k != NULL)
	{
		*e = 0;
		return k;
	}
	*e = ERR_get_error();
	return NULL;
}

// d2i_EC_PUBKEY + ERR_get_error in a single function.
EC_KEY *openssl_d2i_EC_PUBKEY(const unsigned char *data, long len, unsigned long *e)
{
	EC_KEY *k = d2i_EC_PUBKEY(NULL, &data, len);
	if (k != NULL)
	{
		*e = 0;
		return k;
	}
	*e = ERR_get_error();
	return NULL;
}

// d2i_RSAPrivateKey + ERR_get_error in a single function.
RSA *openssl_d2i_RSAPrivateKey(const unsigned char *data, long len, unsigned long *e)
{
	RSA *k = d2i_RSAPrivateKey(NULL, &data, len);
	if (k != NULL)
	{
		*e = 0;
		return k;
	}
	*e = ERR_get_error();
	return NULL;
}

// d2i_RSA_PUBKEY + ERR_get_error in a single function.
RSA *openssl_d2i_RSA_PUBKEY(const unsigned char *data, long len, unsigned long *e)
{
	RSA *k = d2i_RSA_PUBKEY(NULL, &data, len);
	if (k != NULL)
	{
		*e = 0;
		return k;
	}
	*e = ERR_get_error();
	return NULL;
}

// ECDSA_do_sign + ERR_get_error in a single function.
ECDSA_SIG *openssl_ECDSA_do_sign(const unsigned char *digest, int len, EC_KEY *key, unsigned long *e)
{
	ECDSA_SIG *sig = ECDSA_do_sign(digest, len, key);
	if (sig != NULL)
	{
		*e = 0;
		return sig;
	}
	*e = ERR_get_error();
	return NULL;
}

unsigned long openssl_EVP_sign_oneshot(EVP_PKEY *key, EVP_MD *dt, const unsigned char *digest, size_t digestLen, unsigned char *sig, size_t siglen)
{
	EVP_MD_CTX *ctx = EVP_MD_CTX_new();
	if (EVP_DigestSignInit(ctx, NULL, dt, NULL, key) <= 0)
	{
		goto err;
	}
	if (EVP_DigestSign(ctx, sig, &siglen, digest, digestLen) <= 0)
	{
		goto err;
	}
err:
	EVP_MD_CTX_free(ctx);
	return ERR_get_error();
}

unsigned long openssl_EVP_sign(EVP_PKEY *key, EVP_MD *dt, const unsigned char *digest, size_t digestLen, unsigned char **sig, size_t *siglen)
{
	EVP_MD_CTX *ctx = EVP_MD_CTX_new();
	if (EVP_DigestSignInit(ctx, NULL, dt, NULL, key) <= 0)
	{
		goto err;
	}
	if (EVP_DigestSignUpdate(ctx, digest, digestLen) <= 0)
	{
		goto err;
	}
	if (EVP_DigestSignFinal(ctx, NULL, siglen) <= 0)
	{
		goto err;
	}
	*sig = OPENSSL_zalloc(*siglen);
	if (sig == NULL)
	{
		goto err;
	}
	if (EVP_DigestSignFinal(ctx, *sig, siglen) <= 0)
	{
		goto err;
	}
	EVP_MD_CTX_free(ctx);
	return 0;
err:
	EVP_MD_CTX_free(ctx);
	return ERR_get_error();
}

int openssl_EVP_verify(EVP_PKEY *key, EVP_MD *dt, const unsigned char *digest, size_t digestLen, unsigned char *sig, size_t siglen, unsigned long *e)
{
	EVP_MD_CTX *ctx = EVP_MD_CTX_new();
	int rc = EVP_DigestVerifyInit(ctx, NULL, dt, NULL, key);
	if (rc <= 0)
	{
		goto err;
	}
	rc = EVP_DigestVerify(ctx, sig, siglen, digest, digestLen);
	if (rc <= 0)
	{
		goto err;
	}
	EVP_MD_CTX_free(ctx);
	return rc;
err:
	EVP_MD_CTX_free(ctx);
	*e = ERR_get_error();
	return rc;
}