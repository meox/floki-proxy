// Copyright 2021 Gian Lorenzo Meocci (glmeocci@gmail.com). All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.
// Run: ./floki-proxy -failure-rate=10 -fail-with-prefix="/small3/aaa"

package main

import (
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	port           int
	failureRate    int
	failWithPrefix string
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	if shouldFail() {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request to: %s", r.RequestURI)
		return
	}

	if shouldFailByPrefix(r.URL.Path) {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request due to prefix match: %s", r.RequestURI)
		return
	}

	ctx := r.Context()

	cl, f, err := storeBody(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Warnf("failing request due to prefix match: %s", r.RequestURI)
		return
	}
	defer func() {
		err = os.Remove(f.Name())
		if err != nil {
			log.Errorf("cannot remove file %s: %v", f.Name(), err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, r.Method, r.RequestURI, f)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("creating request: %v", err)
		return
	}

	// attach the original headers
	req.Header = r.Header.Clone()
	req.ContentLength = cl
	req.Header.Set("Via", "floki proxy")
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)
	req.Header.Set("X-Forwarded-Host", r.Host)

	// perform the actual request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Errorf("performing the request: %v", err)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		_, errW := w.Write(buf[0:n])
		if errW != nil {
			break
		}
		if err != nil {
			break
		}
	}

	log.WithField("code", resp.Status).
		WithField("method", r.Method).
		WithField("req-bytes", req.ContentLength).
		WithField("resp-bytes", resp.ContentLength).
		Infof("request to %s completed", r.RequestURI)
}

func main() {
	seedRandom()

	flag.IntVar(&port, "port", 9005, "proxy port")
	flag.IntVar(&failureRate, "failure-rate", 0, "percentage of failure")
	flag.StringVar(&failWithPrefix, "fail-with-prefix", "", "fail all request with the given prefix")
	flag.Parse()

	if failureRate < 0 || failureRate > 100 {
		log.Fatal("bad failure rate: expected a value in the range [0, 100]")
	}

	log.Infof("============== STARTING FLOKI PROXY ==================")
	log.Infof("== Listening on: *:%d", port)
	log.Infof("== F-Rate:   %d%%", failureRate)
	log.Infof("== F-Prefix: %s", failWithPrefix)
	log.Infof("======================================================")

	http.HandleFunc("/", mainHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

//shouldFail is an utility function the takes as input the failure-rate
//and using a normal distribution decide if the request should
//fails, returning immediately 500, or should be forwarded
func shouldFail() bool {
	if failureRate == 0 {
		return false
	}
	if failureRate == 100 {
		return true
	}

	return mathrand.Intn(100) < failureRate
}

//shouldFailByPrefix if failure by prefix is set return true if the request path
//match the desired prefix, otherwise return false
func shouldFailByPrefix(path string) bool {
	if failWithPrefix == "" {
		return false
	}

	return strings.HasPrefix(path, failWithPrefix)
}

// seed the random engine using the "/dev/random" as a source
func seedRandom() {
	var r [8]byte
	_, err := rand.Read(r[:])
	if err != nil {
		log.Fatal(err)
	}

	data := binary.BigEndian.Uint64(r[:])
	mathrand.Seed(int64(data))
}

// storeBody: create a temporary file that contains the full request performed by the client
// this is useful in order to set the ContentLength for the forwarding request
func storeBody(body io.Reader) (int64, *os.File, error) {
	f, err := ioutil.TempFile("", "floki-*")
	if err != nil {
		return 0, nil, fmt.Errorf("error opening file: %v", err)
	}

	closeFn := func() {
		_ = f.Close()
		err := os.Remove(f.Name())
		if err != nil {
			log.Errorf("cannot remove %s: %v", f.Name(), err)
		}
	}

	buf := make([]byte, 1*1024*1024)
	cl, err := io.CopyBuffer(f, body, buf)
	if err != nil {
		closeFn()
		return 0, nil, fmt.Errorf("writing file: %w", err)
	}

	err = f.Sync()
	if err != nil {
		closeFn()
		return 0, nil, fmt.Errorf("syncing file: %w", err)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		closeFn()
		return 0, nil, fmt.Errorf("closing file: %w", err)
	}

	return cl, f, nil
}
