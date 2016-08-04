// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package exec

import "os"

// WriteConfigToEnv serializes the supplied Config to the environment
// variable V23_EXEC_CONFIG and appends it to the supplied environment
// slice.
func WriteConfigToEnv(config Config, env []string) ([]string, error) {
	val, err := EncodeForEnvVar(config)
	if err != nil {
		return nil, err
	}
	return append(env, V23_EXEC_CONFIG+"="+val), nil
}

// ReadConfigFromOSEnv deserializes a Config from the environment
// variable V23_EXEC_CONFIG and returns that Config.
func ReadConfigFromOSEnv() (Config, error) {
	str := os.Getenv(V23_EXEC_CONFIG)
	if str == "" {
		return nil, nil
	}
	config := NewConfig()
	return config, DecodeFromEnvVar(str, config)
}
