// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dbutil implements utilities for opening and configuring connections
// to MySQL-like databases, with optional TLS support.
//
// Functions in this file are not thread-safe. However, the returned *sql.DB is.
// Sane defaults are assumed: utf8mb4 encoding, UTC timezone, parsing date/time
// into time.Time.
package dbutil

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// SQL statement suffix to be appended when creating tables.
const SQLCreateTableSuffix = "CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci"

// Description of the SQL configuration file format.
const SQLConfigFileDescription = `File must contain a JSON object of the following form:
   {
    "dataSourceName": "[username[:password]@][protocol[(address)]]/dbname", (the connection string required by go-sql-driver; database name must be specified, query parameters are not supported)
    "tlsDisable": "false|true", (defaults to false; if set to true, uses an unencrypted connection; otherwise, the following fields are mandatory)
    "tlsServerName": "serverName", (the domain name of the SQL server for TLS)
    "rootCertPath": "[/]path/server-ca.pem", (the root certificate of the SQL server for TLS)
    "clientCertPath": "[/]path/client-cert.pem", (the client certificate for TLS)
    "clientKeyPath": "[/]path/client-key.pem" (the client private key for TLS)
   }
Paths must be either absolute or relative to the configuration file directory.`

// SQLConfig holds the fields needed to connect to a SQL instance and to
// configure TLS encryption of the information sent over the wire. It must be
// activated via Activate() before use.
type SQLConfig struct {
	// DataSourceName is the connection string as required by go-sql-driver:
	// "[username[:password]@][protocol[(address)]]/dbname";
	// database name must be specified, query parameters are not supported.
	DataSourceName string `json:"dataSourceName"`
	// TLSDisable, if set to true, uses an unencrypted connection;
	// otherwise, the following fields are mandatory.
	TLSDisable bool `json:"tlsDisable"`
	// TLSServerName is the domain name of the SQL server for TLS.
	TLSServerName string `json:"tlsServerName"`
	// RootCertPath is the root certificate of the SQL server for TLS.
	RootCertPath string `json:"rootCertPath"`
	// ClientCertPath is the client certificate for TLS.
	ClientCertPath string `json:"clientCertPath"`
	// ClientKeyPath is the client private key for TLS.
	ClientKeyPath string `json:"clientKeyPath"`
}

// ActiveSQLConfig represents a SQL configuration that has been activated
// by registering the TLS configuration (if applicable). It can be used for
// opening SQL database connections.
type ActiveSQLConfig struct {
	// cfg is a copy of the SQLConfig that was activated.
	cfg *SQLConfig
	// tlsConfigIdentifier is the identifier under which the TLS configuration
	// is registered with go-sql-driver. It is computed as a secure hash of the
	// SqlConfig after resolving any relative paths.
	tlsConfigIdentifier string
}

// Parses the SQL configuration file pointed to by sqlConfigFile (format
// described in SqlConfigFileDescription; also see links below).
// https://github.com/go-sql-driver/mysql/#dsn-data-source-name
// https://github.com/go-sql-driver/mysql/#tls
func ParseSQLConfigFromFile(sqlConfigFile string) (*SQLConfig, error) {
	configJSON, err := ioutil.ReadFile(sqlConfigFile)
	if err != nil {
		return nil, fmt.Errorf("failed reading SQL config file %q: %v", sqlConfigFile, err)
	}
	var config SQLConfig
	if err = json.Unmarshal(configJSON, &config); err != nil {
		// TODO(ivanpi): Parsing errors might leak the SQL password into error
		// logs, depending on standard library implementation.
		return nil, fmt.Errorf("failed parsing SQL config file %q: %v", sqlConfigFile, err)
	}
	return &config, nil
}

// Activates the SQL configuration by registering the TLS configuration with
// go-mysql-driver (if TLSDisable is not set).
// Certificate paths from SqlConfig that aren't absolute are interpreted relative
// to certBaseDir.
// For more information see https://github.com/go-sql-driver/mysql/#tls
func (sc *SQLConfig) Activate(certBaseDir string) (*ActiveSQLConfig, error) {
	if sc.TLSDisable {
		return &ActiveSQLConfig{
			cfg: sc.normalizePaths(""),
		}, nil
	}
	cbdAbs, err := filepath.Abs(certBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed resolving certificate base directory %q: %v", certBaseDir, err)
	}
	scn := sc.normalizePaths(cbdAbs)
	configID := scn.hash()
	if err = registerSQLTLSConfig(scn, configID); err != nil {
		return nil, fmt.Errorf("failed registering TLS config: %v", err)
	}
	return &ActiveSQLConfig{
		cfg:                 scn,
		tlsConfigIdentifier: configID,
	}, nil
}

// Convenience function to parse and activate the SQL configuration file.
// Certificate paths that aren't absolute are interpreted relative to the
// directory containing sqlConfigFile.
func ActivateSQLConfigFromFile(sqlConfigFile string) (*ActiveSQLConfig, error) {
	cfg, err := ParseSQLConfigFromFile(sqlConfigFile)
	if err != nil {
		return nil, err
	}
	activeCfg, err := cfg.Activate(filepath.Dir(sqlConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed activating SQL config from file %q: %v", sqlConfigFile, err)
	}
	return activeCfg, nil
}

// Opens a connection to the SQL database using the provided configuration.
// Sets the specified transaction isolation (see link below).
// https://dev.mysql.com/doc/refman/5.5/en/server-system-variables.html#sysvar_tx_isolation
func (sqlConfig *ActiveSQLConfig) NewSQLDBConn(txIsolation string) (*sql.DB, error) {
	return openSQLDBConn(configureSQLDBConn(sqlConfig, txIsolation))
}

// Convenience function to parse and activate the configuration file and open
// a connection to the SQL database. If multiple connections with the same
// configuration are needed, a single ActivateSQLConfigFromFile() and multiple
// NewSQLDbConn() calls are recommended instead.
func NewSQLDBConnFromFile(sqlConfigFile, txIsolation string) (*sql.DB, error) {
	config, err := ActivateSQLConfigFromFile(sqlConfigFile)
	if err != nil {
		return nil, err
	}
	return config.NewSQLDBConn(txIsolation)
}

func configureSQLDBConn(sqlConfig *ActiveSQLConfig, txIsolation string) string {
	params := url.Values{}
	// Setting charset is unnecessary when collation is set, according to
	// https://github.com/go-sql-driver/mysql/#charset
	params.Set("collation", "utf8mb4_general_ci")
	// Maps SQL date/time values into time.Time instead of strings.
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	params.Set("time_zone", "'+00:00'")
	if !sqlConfig.cfg.TLSDisable {
		params.Set("tls", sqlConfig.tlsConfigIdentifier)
	}
	params.Set("tx_isolation", "'"+txIsolation+"'")
	return sqlConfig.cfg.DataSourceName + "?" + params.Encode()
}

func openSQLDBConn(dataSrcName string) (*sql.DB, error) {
	// Prevent leaking the SQL password into error logs.
	sanitizedDSN := dataSrcName[strings.LastIndex(dataSrcName, "@")+1:]
	db, err := sql.Open("mysql", dataSrcName)
	if err != nil {
		return nil, fmt.Errorf("failed opening database connection at %q: %v", sanitizedDSN, err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed connecting to database at %q: %v", sanitizedDSN, err)
	}
	return db, nil
}

// registerSQLTLSConfig sets up the SQL connection to use TLS encryption
// and registers the configuration under configID.
// For more information see https://github.com/go-sql-driver/mysql/#tls
func registerSQLTLSConfig(cfg *SQLConfig, configID string) error {
	rootCertPool := x509.NewCertPool()
	pem, err := ioutil.ReadFile(cfg.RootCertPath)
	if err != nil {
		return fmt.Errorf("failed reading root certificate in %v: %v", cfg.RootCertPath, err)
	}
	if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
		return fmt.Errorf("failed to add root certificate in %v to cert pool", cfg.RootCertPath)
	}
	ckpair, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
	if err != nil {
		return fmt.Errorf("failed loading client key pair (%v, %v): %v", cfg.ClientCertPath, cfg.ClientKeyPath, err)
	}
	clientCert := []tls.Certificate{ckpair}
	return mysql.RegisterTLSConfig(configID, &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCert,
		ServerName:   cfg.TLSServerName,
		// SSLv3 is more vulnerable than TLSv1.0, see https://en.wikipedia.org/wiki/POODLE
		// TODO(ivanpi): Increase when Cloud SQL starts supporting higher TLS versions.
		MinVersion: tls.VersionTLS10,
		ClientAuth: tls.RequireAndVerifyClientCert,
	})
}

// Computes a secure hash of the SqlConfig. Paths are canonicalized before
// hashing.
func (sc *SQLConfig) hash() string {
	scn := sc.normalizePaths("")
	fieldsToHash := []interface{}{
		scn.DataSourceName, scn.TLSDisable, scn.TLSServerName,
		scn.RootCertPath, scn.ClientCertPath, scn.ClientKeyPath,
	}
	hashAcc := make([]byte, 0, len(fieldsToHash)*sha256.Size)
	for _, field := range fieldsToHash {
		fieldHash := sha256.Sum256([]byte(fmt.Sprintf("%v", field)))
		hashAcc = append(hashAcc, fieldHash[:]...)
	}
	structHash := sha256.Sum256(hashAcc)
	return hex.EncodeToString(structHash[:])
}

// Returns a copy of the SQLConfig with certificate paths canonicalized and
// resolved relative to certBaseDir. Blank paths remain blank.
func (sc *SQLConfig) normalizePaths(certBaseDir string) *SQLConfig {
	scn := *sc
	for _, path := range []*string{&scn.RootCertPath, &scn.ClientCertPath, &scn.ClientKeyPath} {
		*path = normalizePath(*path, certBaseDir)
	}
	return &scn
}

// If path is not absolute, resolves path relative to baseDir. Otherwise,
// canonicalizes path. Blank paths are not resolved or canonicalized.
func normalizePath(path, baseDir string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(baseDir, path)
}
