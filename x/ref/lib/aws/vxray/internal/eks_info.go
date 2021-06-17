package internal

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

const (
	k8sServiceAccountPrefix = "/var/run/secrets/kubernetes.io/serviceaccount"
	k8sCert                 = "ca.crt"
	k8sToken                = "token"
	k8sAPIHost              = "kubernetes.default.svc"
	kContainerIDLen         = 64
)

func GetEKSClusterName(ctx context.Context, configMap, keyName string) (string, error) {
	cm, err := getConfigMap(ctx, configMap)
	if err != nil {
		return "", err
	}
	name, ok := cm[keyName]
	if !ok {
		return "", fmt.Errorf("cluster name key %v not found", keyName)
	}
	return name, nil
}

func getConfigMap(ctx context.Context, configMap string) (map[string]string, error) {
	rootPEM, err := ioutil.ReadFile(filepath.Join(k8sServiceAccountPrefix, k8sCert))
	if err != nil {
		return nil, err
	}
	token, err := ioutil.ReadFile(filepath.Join(k8sServiceAccountPrefix, k8sToken))
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		panic("failed to parse root certificate")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: roots,
		},
	}
	client := &http.Client{Transport: tr}
	u := &url.URL{
		Scheme: "https",
		Host:   k8sAPIHost,
		Path:   configMap,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	io.Copy(buf, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: ERROR: %v", buf.String(), resp.StatusCode)
	}
	cm := struct {
		API  string            `json:"apiVersion"`
		Data map[string]string `json:"data"`
	}{}
	if err := json.Unmarshal(buf.Bytes(), &cm); err != nil {
		return nil, fmt.Errorf("%s: %v", buf.String(), err)
	}
	if cm.API != "v1" && len(cm.Data) == 0 {
		return nil, fmt.Errorf("API version has changed to %v: found no config map data", cm.API)

	}
	return cm.Data, nil
}

func GetContainerID(cgroupFile string) (string, error) {
	rd, err := os.Open(cgroupFile)
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(rd)
	cid := ""
	for sc.Scan() {
		line := sc.Text()
		if l := len(line); l > kContainerIDLen {
			cid = line[l-kContainerIDLen:]
			break
		}
	}
	if len(cid) == 0 {
		return "", fmt.Errorf("failed to find a container id in %v", cgroupFile)
	}
	return cid, nil
}
