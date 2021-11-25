package braidhttp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"go.uber.org/multierr"
	"golang.org/x/net/publicsuffix"

	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/errors"
	"redwood.dev/identity"
	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/redwood.js/embed/redwoodjs"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protoauth"
	"redwood.dev/swarm/protoblob"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/tree/nelson"
	"redwood.dev/types"
	"redwood.dev/utils"
)

type Transport interface {
	process.Interface
	swarm.Transport
	protoauth.AuthTransport
	protoblob.BlobTransport
	prototree.TreeTransport
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type transport struct {
	process.Process
	log.Logger
	protoauth.BaseAuthTransport
	protoblob.BaseBlobTransport
	prototree.BaseTreeTransport

	controllerHub   tree.ControllerHub
	defaultStateURI string
	ownURLs         types.StringSet
	listenAddr      string
	listenAddrSSL   string
	cookieSecret    [32]byte
	tlsCertFilename string
	tlsKeyFilename  string
	tlsCerts        []tls.Certificate
	cookieJar       http.CookieJar
	jwtSecret       []byte
	devMode         bool

	srv        *http.Server
	srvTLS     *http.Server
	httpClient *utils.HTTPClient

	pendingAuthorizations map[types.ID][]byte

	treeACL   prototree.ACL
	peerStore swarm.PeerStore
	keyStore  identity.KeyStore
	blobStore blob.Store
}

var _ Transport = (*transport)(nil)

const (
	TransportName string = "braidhttp"
)

func NewTransport(
	listenAddr string,
	listenAddrSSL string,
	ownURLs types.StringSet,
	defaultStateURI string,
	controllerHub tree.ControllerHub,
	keyStore identity.KeyStore,
	blobStore blob.Store,
	peerStore swarm.PeerStore,
	tlsCertFilename, tlsKeyFilename string,
	tlsCerts []tls.Certificate,
	jwtSecret []byte,
	devMode bool,
) (*transport, error) {
	t := &transport{
		Process:               *process.New(TransportName),
		Logger:                log.NewLogger(TransportName),
		controllerHub:         controllerHub,
		listenAddr:            listenAddr,
		listenAddrSSL:         listenAddrSSL,
		defaultStateURI:       defaultStateURI,
		tlsCertFilename:       tlsCertFilename,
		tlsKeyFilename:        tlsKeyFilename,
		tlsCerts:              tlsCerts,
		jwtSecret:             jwtSecret,
		devMode:               devMode,
		pendingAuthorizations: make(map[types.ID][]byte),
		ownURLs:               ownURLs,
		keyStore:              keyStore,
		blobStore:             blobStore,
		peerStore:             peerStore,
	}
	return t, nil
}

func (t *transport) Start() error {
	err := t.Process.Start()
	if err != nil {
		return err
	}

	t.Infof(0, "opening %v transport at %v", TransportName, t.listenAddr)
	t.Infof(0, "opening %v transport (ssl) at %v", TransportName, t.listenAddrSSL)

	t.cookieJar, err = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}

	if len(t.listenAddrSSL) > 0 {
		if t.listenAddrSSL[0] == ':' {
			t.ownURLs.Add("https://localhost" + t.listenAddrSSL)
		} else {
			t.ownURLs.Add("https://" + t.listenAddrSSL)
		}
	}

	t.httpClient = utils.MakeHTTPClient(10*time.Second, 30*time.Second, t.cookieJar, t.tlsCerts)

	err = t.findOrCreateCookieSecret()
	if err != nil {
		return err
	}

	// Update our node's info in the peer store
	identities, err := t.keyStore.Identities()
	if err != nil {
		return err
	}
	for _, identity := range identities {
		for ownURL := range t.ownURLs {
			t.peerStore.AddVerifiedCredentials(
				swarm.PeerDialInfo{TransportName: TransportName, DialAddr: ownURL},
				"self",
				identity.SigKeypair.SigningPublicKey.Address(),
				identity.SigKeypair.SigningPublicKey,
				identity.AsymEncKeypair.AsymEncPubkey,
			)
		}
	}

	t.Process.Go(nil, "http server", func(ctx context.Context) {
		t.srvTLS = &http.Server{
			Addr:    t.listenAddrSSL,
			Handler: utils.UnrestrictedCors(t),
			TLSConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				MaxVersion:         tls.VersionTLS13,
				Certificates:       t.tlsCerts,
				ClientAuth:         tls.RequestClientCert,
				InsecureSkipVerify: true,
			},
		}
		go func() {
			t.Infof(0, "tls cert file: %v", t.tlsCertFilename)
			t.Infof(0, "tls key file: %v", t.tlsKeyFilename)
			err := t.srvTLS.ListenAndServeTLS(t.tlsCertFilename, t.tlsKeyFilename)
			if err != nil {
				t.Errorf("while starting HTTPS server: %v", err)
			}
		}()

		t.srv = &http.Server{
			Addr:    t.listenAddr,
			Handler: utils.UnrestrictedCors(t),
		}
		go func() {
			err := t.srv.ListenAndServe()
			if err != nil {
				t.Errorf("while starting HTTP server: %v", err)
			}
		}()
	})

	return nil
}

func (t *transport) Close() error {
	// Non-graceful
	err := t.srv.Close()
	if err != nil {
		t.Errorf("error closing http server: %v", err)
	}
	// Graceful
	// err = t.srv.Shutdown(context.Background())
	// if err != nil {
	//  fmt.Println(err)
	// }

	t.httpClient.Close()
	return t.Process.Close()
}

func (t *transport) Name() string {
	return TransportName
}

func (t *transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if dump, _ := strconv.ParseBool(os.Getenv("HTTP_DUMP_REQUESTS")); dump {
		reqBytes, err := httputil.DumpRequest(r, true)
		if err != nil {
			t.Errorf("error dumping request body: %v", err)
		} else {
			t.Debugf("incoming HTTP request:\n%v", string(reqBytes))
		}
	}

	sessionID, err := t.ensureSessionIDCookieOnResponse(w, r)
	if err != nil {
		t.Errorf("error reading sessionID cookie: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	address, err := t.addressFromRequest(r)
	if err != nil {
		t.Errorf("err: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	duID := sessionID.Hex()
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		duID = utils.DeviceIDFromX509Pubkey(r.TLS.PeerCertificates[0].PublicKey)
	}
	peerConn := t.makePeerConn(w, nil, "", sessionID, duID, address)

	// Peer discovery
	{
		// On every incoming request, advertise other peers via the Alt-Svc header
		altSvcHeader := makeAltSvcHeader(t.peerStore.AllDialInfos())
		w.Header().Set("Alt-Svc", altSvcHeader)

		// Similarly, if other peers give us Alt-Svc headers, track them
		t.storeAltSvcHeaderPeers(r.Header)
	}

	switch r.Method {
	case "HEAD":
		// This is mainly used to poll for new peers

	case "OPTIONS":
		// w.Header().Set("Access-Control-Allow-Headers", "State-URI")

	case "AUTHORIZE":
		// Address verification requests (2 kinds)
		if challengeMsgHex := r.Header.Get("Challenge"); challengeMsgHex != "" {
			// 1. Remote node is reaching out to us with a challenge, and we respond
			t.serveChallengeIdentityResponse(w, r, peerConn, challengeMsgHex)
		} else {
			responseHex := r.Header.Get("Response")
			if responseHex == "" {
				// 2a. Remote node wants a challenge, so we send it
				t.serveChallengeIdentity(w, r, peerConn)
			} else {
				// 2b. Remote node wanted a challenge, this is their response
				t.serveChallengeIdentityCheckResponse(w, r, peerConn, responseHex)
			}
		}

	case "GET":
		if r.URL.Path == "/ws" {
			t.serveWSSubscription(w, r, sessionID, address)
		} else if r.Header.Get("Subscribe") != "" {
			t.serveHTTPSubscription(w, r, sessionID, address)
		} else {
			if r.URL.Path == "/redwood.js" {
				// @@TODO: this is hacky
				t.serveRedwoodJS(w, r)
			} else if strings.HasPrefix(r.URL.Path, "/__tx/") {
				t.serveGetTx(w, r)
			} else {
				t.serveGetState(w, r)
			}
		}

	case "POST":
		if r.Header.Get("Blob") == "true" {
			t.servePostBlob(w, r)
		}

	case "ACK":
		t.serveAck(w, r, peerConn)

	case "PUT":
		if r.Header.Get("Private") == "true" {
			t.servePostPrivateTx(w, r, peerConn)
		} else {
			t.servePostTx(w, r, peerConn)
		}

	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (t *transport) findOrCreateCookieSecret() error {
	maybeSecret, exists, err := t.keyStore.ExtraUserData("http:cookiesecret")
	if err != nil {
		return err
	}

	cookieSecret, isBytes := maybeSecret.([]byte)
	if !exists || !isBytes {
		cookieSecret := make([]byte, 32)
		_, err := rand.Read(cookieSecret)
		if err != nil {
			return err
		}
	}
	copy(t.cookieSecret[:], cookieSecret)
	return t.keyStore.SaveExtraUserData("http:cookiesecret", t.cookieSecret)
}

func (t *transport) storeAltSvcHeaderPeers(h http.Header) {
	if altSvcHeader := h.Get("Alt-Svc"); altSvcHeader != "" {
		forEachAltSvcHeaderPeer(altSvcHeader, func(transportName, dialAddr string, metadata map[string]string) {
			t.peerStore.AddDialInfo(swarm.PeerDialInfo{transportName, dialAddr}, "")
		})
	}
}

func (t *transport) addressFromRequest(r *http.Request) (types.Address, error) {
	type request struct {
		JWT string `header:"Authorization" query:"ucan"`
	}
	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		return types.Address{}, err
	}

	claims, exists, err := utils.ParseJWT(req.JWT, t.jwtSecret)
	if err != nil {
		return types.Address{}, err
	} else if !exists {
		claims, exists, err = utils.ParseJWT(req.JWT, t.jwtSecret)
		if err != nil {
			return types.Address{}, err
		}
	}
	if exists {
		addrHex, ok := claims["address"].(string)
		if !ok {
			return types.Address{}, errors.Errorf("jwt does not contain 'address' claim")
		}
		addr, err := types.AddressFromHex(addrHex)
		if err != nil {
			return types.Address{}, errors.Wrapf(err, "jwt 'address' claim contains invalid data")
		}
		return addr, nil
	}

	addressBytes, err := t.signedCookie(r, "address")
	if errors.Cause(err) == errors.Err404 {
		return types.Address{}, nil
	} else if err != nil {
		return types.Address{}, err
	}
	return types.AddressFromBytes(addressBytes), nil
}

// Respond to a request from another node challenging our identity.
func (t *transport) serveChallengeIdentityResponse(w http.ResponseWriter, r *http.Request, peerConn *peerConn, challengeMsgHex string) {
	challengeMsg, err := hex.DecodeString(challengeMsgHex)
	if err != nil {
		http.Error(w, "Challenge header: bad challenge message", http.StatusBadRequest)
		return
	}

	err = t.HandleChallengeIdentity([]byte(challengeMsg), peerConn)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Respond to a request from another node that wants us to issue them an identity
// challenge.  This is used with browser nodes which we cannot reach out to directly.
func (t *transport) serveChallengeIdentity(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	challenge, err := protoauth.GenerateChallengeMsg()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t.pendingAuthorizations[peerConn.sessionID] = challenge

	challengeHex := hex.EncodeToString(challenge)
	_, err = w.Write([]byte(challengeHex))
	if err != nil {
		t.Errorf("error writing challenge message: %v", err)
		return
	}
}

// Check the response from another node that requested an identity challenge from us.
// This is used with browser nodes which we cannot reach out to directly.
func (t *transport) serveChallengeIdentityCheckResponse(w http.ResponseWriter, r *http.Request, peerConn *peerConn, responseHex string) {
	challenge, exists := t.pendingAuthorizations[peerConn.sessionID]
	if !exists {
		http.Error(w, "no pending authorization", http.StatusBadRequest)
		return
	}

	sig, err := hex.DecodeString(responseHex)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sigpubkey, err := crypto.RecoverSigningPubkey(types.HashBytes(challenge), sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	addr := sigpubkey.Address()
	err = t.setSignedCookieOnResponse(w, "address", addr[:])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// @@TODO: make the request include the encrypting pubkey as well
	t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName, ""}, peerConn.DeviceUniqueID(), addr, sigpubkey, nil)

	delete(t.pendingAuthorizations, peerConn.sessionID) // @@TODO: expiration/garbage collection for failed auths
}

func (t *transport) serveHTTPSubscription(w http.ResponseWriter, r *http.Request, sessionID types.ID, address types.Address) {
	type request struct {
		StateURI string                     `header:"State-URI" query:"state_uri"         required:"true"`
		Keypath  state.Keypath              `header:"Keypath"   query:"keypath"`
		SubType  prototree.SubscriptionType `header:"Subscribe" query:"subscription_type" required:"true"`
		FromTxID *state.Version             `header:"From-Tx"   query:"from_tx"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// @@TODO: ensure we actually have this stateURI?
	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	t.Infof(0, "incoming http subscription (address: %v, state uri: %v)", address, req.StateURI)

	var fetchHistoryOpts prototree.FetchHistoryOpts
	if req.FromTxID != nil {
		fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: *req.FromTxID}
	}

	subRequest := prototree.SubscriptionRequest{
		StateURI:         req.StateURI,
		Keypath:          state.Keypath(req.Keypath),
		Type:             req.SubType,
		FetchHistoryOpts: &fetchHistoryOpts,
		Addresses:        types.NewAddressSet([]types.Address{address}),
	}
	chSubClosed, err := t.HandleWritableSubscriptionOpened(subRequest, func() (prototree.WritableSubscriptionImpl, error) {
		return newHTTPWritableSubscription(req.StateURI, w, r), nil
	})
	if errors.Cause(err) == errors.Err403 {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	} else if errors.Cause(err) == errors.Err404 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Block until the subscription is canceled so that net/http doesn't close the connection
	select {
	case <-chSubClosed:
	case <-t.Process.Done():
	}
}

func (t *transport) serveWSSubscription(w http.ResponseWriter, r *http.Request, sessionID types.ID, address types.Address) {
	type request struct {
		StateURI string                     `header:"State-URI" query:"state_uri"         required:"true"`
		Keypath  state.Keypath              `header:"Keypath"   query:"keypath"`
		SubType  prototree.SubscriptionType `header:"Subscribe" query:"subscription_type" required:"true"`
		FromTxID *state.Version             `header:"From-Tx"   query:"from_tx"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	var fetchHistoryOpts prototree.FetchHistoryOpts
	if req.FromTxID != nil {
		fetchHistoryOpts = prototree.FetchHistoryOpts{FromTxID: *req.FromTxID}
	}

	subRequest := prototree.SubscriptionRequest{
		StateURI:         req.StateURI,
		Keypath:          state.Keypath(req.Keypath),
		Type:             req.SubType,
		FetchHistoryOpts: &fetchHistoryOpts,
		Addresses:        types.NewAddressSet([]types.Address{address}),
	}
	chSubClosed, err := t.HandleWritableSubscriptionOpened(subRequest, func() (prototree.WritableSubscriptionImpl, error) {
		wsConn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return nil, err
		}
		return newWSWritableSubscription(req.StateURI, wsConn, []types.Address{address}, t), nil
	})
	if errors.Cause(err) == errors.Err403 {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	} else if errors.Cause(err) == errors.Err404 {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Block until the subscription is canceled so that net/http doesn't close the connection
	select {
	case <-chSubClosed:
	case <-t.Process.Done():
	}
}

func (t *transport) serveRedwoodJS(w http.ResponseWriter, r *http.Request) {
	http.ServeContent(w, r, "./redwood.js", time.Now(), bytes.NewReader(redwoodjs.BrowserSrc))
}

func (t *transport) serveGetTx(w http.ResponseWriter, r *http.Request) {
	type request struct {
		StateURI string `header:"State-URI" query:"state_uri" required:"true"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parts := strings.Split(r.URL.Path[1:], "/")
	txIDStr := parts[1]
	txID, err := state.VersionFromHex(txIDStr)
	if err != nil {
		http.Error(w, "bad tx id", http.StatusBadRequest)
		return
	}

	tx, err := t.controllerHub.FetchTx(req.StateURI, txID)
	if err != nil {
		http.Error(w, fmt.Sprintf("not found: %v", err), http.StatusNotFound)
		return
	}

	utils.RespondJSON(w, tx)
}

func (t *transport) serveGetState(w http.ResponseWriter, r *http.Request) {
	type request struct {
		StateURI string         `header:"State-URI" query:"state_uri"`
		Version  *state.Version `header:"Version"`
	}

	var req request
	err := utils.UnmarshalHTTPRequest(&req, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.StateURI == "" {
		req.StateURI = t.defaultStateURI
	}

	keypathStrs := utils.FilterEmptyStrings(strings.Split(r.URL.Path[1:], "/"))
	keys := r.URL.Query()
	fileName := keys.Get("filename")

	if len(fileName) != 0 {
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	}

	keypathStr := strings.Join(keypathStrs, string(state.KeypathSeparator))
	keypath := state.Keypath(keypathStr)
	parts := keypath.Parts()
	newParts := make([]state.Keypath, 0, len(parts))
	for _, part := range parts {
		if idx := part.IndexByte('['); idx > -1 {
			newParts = append(newParts, part[:idx])
			x, err := strconv.ParseInt(string(part[idx+1:len(part)-1]), 10, 64)
			if err != nil {
				http.Error(w, "bad slice index", http.StatusBadRequest)
				return
			}
			newParts = append(newParts, state.EncodeSliceIndex(uint64(x)))
		} else {
			newParts = append(newParts, part)
		}
	}
	keypath = state.JoinKeypaths(newParts, []byte("/"))

	var rng *state.Range
	fmt.Println(r.Header.Get("Range"))
	if rstr := r.Header.Get("Range"); rstr != "" && rstr != "bytes=0-" {
		// Range: -10:-5
		// @@TODO: real json Range parsing
		parts := strings.SplitN(rstr, "=", 2)
		if len(parts) != 2 {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		} else if parts[0] != "json" {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		parts = strings.SplitN(parts[1], ":", 2)
		if len(parts) != 2 {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		start, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		end, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			http.Error(w, "bad Range header", http.StatusBadRequest)
			return
		}
		rng = &state.Range{start, end}
	}

	// Add the "Parents" header
	{
		leaves, err := t.controllerHub.Leaves(req.StateURI)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v", err), http.StatusNotFound)
			return
		}
		if len(leaves) > 0 {
			leaf := leaves[0]

			tx, err := t.controllerHub.FetchTx(req.StateURI, leaf)
			if err != nil {
				http.Error(w, fmt.Sprintf("can't fetch tx %v: %+v", leaf, err.Error()), http.StatusNotFound)
				return
			}
			parents := tx.Parents

			var parentStrs []string
			for _, pid := range parents {
				parentStrs = append(parentStrs, pid.Hex())
			}
			w.Header().Add("Parents", strings.Join(parentStrs, ","))
		} else {
			w.Header().Add("Parents", "")
		}
	}

	indexName, indexArg := parseIndexParams(r)
	raw, err := parseRawParam(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusBadRequest)
		return
	}

	var node state.Node
	var anyMissing bool

	if indexName != "" {
		// Index query

		// You can specify an index_arg of * in order to fetch the entire index
		var indexArgKeypath state.Keypath
		if indexArg != "*" {
			indexArgKeypath = state.Keypath(indexArg)
		}
		node, err = t.controllerHub.QueryIndex(stateURI, version, keypath, state.Keypath(indexName), indexArgKeypath, rng)
		if errors.Cause(err) == types.Err404 {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			return
		}

	} else {
		// State query
		node, err = t.controllerHub.StateAtVersion(req.StateURI, req.Version)
		if err != nil {
			http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			return
		}
		defer node.Close()

		if raw {
			node = node.NodeAt(keypath, rng)
		} else {
			var exists bool
			node, exists, err = nelson.Seek(node, keypath, t.controllerHub, t.blobStore)
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			} else if !exists {
				http.Error(w, fmt.Sprintf("not found: %v", keypath), http.StatusNotFound)
				return
			}
			keypath = nil

			// node, err = node.CopyToMemory(keypath, rng)
			// if errors.Cause(err) == errors.Err404 {
			// 	http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
			// 	return
			// } else if err != nil {
			// 	http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			// 	return
			// }

			// node, anyMissing, err = nelson.Resolve(node, t.controllerHub, t.blobStore)
			// if err != nil {
			// 	http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
			// 	return
			// }

			indexHTMLExists, err := node.Exists(keypath.Push(state.Keypath("index.html")))
			if err != nil {
				http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusInternalServerError)
				return
			}
			if indexHTMLExists {
				keypath = keypath.Push(state.Keypath("index.html"))
				node = node.NodeAt(state.Keypath("index.html"), nil)
			}
		}
	}

	contentType, err := nelson.GetContentType(node)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting content type: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentType == "application/octet-stream" {
		contentType = utils.GuessContentTypeFromFilename(string(keypath.Part(-1)))
	}
	w.Header().Set("Content-Type", contentType)

	contentLength, err := nelson.GetContentLength(node)
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting content length: %+v", err), http.StatusInternalServerError)
		return
	}
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	resourceLength, err := node.Length()
	if err != nil {
		http.Error(w, fmt.Sprintf("error getting length: %+v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Resource-Length", strconv.FormatUint(resourceLength, 10))

	var val interface{}
	var exists bool
	if !raw {
		val, exists, err = nelson.GetValueRecursive(node, nil, nil)
	} else {
		val, exists, err = node.Value(nil, nil)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("error: %+v", err), http.StatusBadRequest)
		return
	} else if !exists {
		http.Error(w, fmt.Sprintf("not found: %+v", err), http.StatusNotFound)
		return
	}

	respBuf, ok := nelson.GetReadCloser(val)
	if !ok {
		contentType = "application/json"
		j, err := json.Marshal(val)
		if err != nil {
			panic(err)
		}
		respBuf = ioutil.NopCloser(bytes.NewBuffer(j))
	}
	defer respBuf.Close()

	// Right now, this is just to facilitate the Chrome extension
	allowSubscribe := map[string]bool{
		"application/json": true,
		"application/js":   true,
		"text/plain":       true,
	}
	if allowSubscribe[contentType] {
		w.Header().Set("Subscribe", "Allow")
	}

	if anyMissing {
		w.WriteHeader(http.StatusPartialContent)
	}

	_, err = io.Copy(w, respBuf)
	if err != nil {
		http.Error(w, "could not copy response", http.StatusInternalServerError)
	}
}

func (t *transport) serveAck(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	defer r.Body.Close()

	bs, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	var txID state.Version
	err = txID.UnmarshalText(bs)
	if err != nil {
		t.Errorf("error reading ACK body: %v", err)
		http.Error(w, "error reading body", http.StatusBadRequest)
		return
	}

	stateURI := r.Header.Get("State-URI")

	t.HandleAckReceived(stateURI, txID, peerConn)
}

func (t *transport) servePostBlob(w http.ResponseWriter, r *http.Request) {
	t.Infof(0, "incoming blob")

	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("blob")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	sha1Hash, sha3Hash, err := t.blobStore.StoreBlob(file)
	if err != nil {
		t.Errorf("error storing blob: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	utils.RespondJSON(w, StoreBlobResponse{SHA1: sha1Hash, SHA3: sha3Hash})
}

func (t *transport) servePostTx(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	type request struct {
		StateURI   string
		Signature  types.Signature
		Version    state.Version
		Parents    []state.Version
		Checkpoint bool
	}

	t.Infof(0, "incoming tx")

	var err error

	var sig types.Signature
	sigHeaderStr := r.Header.Get("Signature")
	if sigHeaderStr == "" {
		http.Error(w, "missing Signature header", http.StatusBadRequest)
		return
	} else {
		sig, err = types.SignatureFromHex(sigHeaderStr)
		if err != nil {
			http.Error(w, "bad Signature header", http.StatusBadRequest)
			return
		}
	}

	var txID state.Version
	txIDStr := r.Header.Get("Version")
	if txIDStr == "" {
		txID = state.RandomVersion()
	} else {
		txID, err = state.VersionFromHex(txIDStr)
		if err != nil {
			http.Error(w, "bad Version header", http.StatusBadRequest)
			return
		}
	}

	var parents []state.Version
	parentsStr := r.Header.Get("Parents")
	if parentsStr != "" {
		parentsStrs := strings.Split(parentsStr, ",")
		for _, pstr := range parentsStrs {
			parentID, err := state.VersionFromHex(strings.TrimSpace(pstr))
			if err != nil {
				http.Error(w, "bad Parents header", http.StatusBadRequest)
				return
			}
			parents = append(parents, parentID)
		}
	}

	var checkpoint bool
	if checkpointStr := r.Header.Get("Checkpoint"); checkpointStr == "true" {
		checkpoint = true
	}

	stateURI := r.Header.Get("State-URI")
	if stateURI == "" {
		stateURI = t.defaultStateURI
	}

	var attachment []byte
	var patchReader io.Reader

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(10000000)
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}

		_, header, err := r.FormFile("attachment")
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}
		file, err := header.Open()
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}
		defer file.Close()

		attachment, err = ioutil.ReadAll(file)
		if err != nil {
			http.Error(w, "error parsing multipart form", http.StatusBadRequest)
			return
		}

		patchesStrs := r.MultipartForm.Value["patches"]
		if len(patchesStrs) > 0 {
			patchReader = strings.NewReader(patchesStrs[0])
		}

	} else {
		patchReader = r.Body
	}

	var patches []tree.Patch
	if patchReader != nil {
		scanner := bufio.NewScanner(patchReader)
		for scanner.Scan() {
			line := scanner.Bytes()
			patch, err := tree.ParsePatch(line)
			if err != nil {
				http.Error(w, fmt.Sprintf("bad patch string: %v", line), http.StatusBadRequest)
				return
			}
			patches = append(patches, patch)
		}
		if err := scanner.Err(); err != nil {
			http.Error(w, fmt.Sprintf("internal server error: %v", err), http.StatusInternalServerError)
			return
		}
	}

	tx := tree.Tx{
		ID:         txID,
		Parents:    parents,
		Sig:        sig,
		Patches:    patches,
		Attachment: attachment,
		StateURI:   stateURI,
		Checkpoint: checkpoint,
	}

	// @@TODO: remove .From entirely
	pubkey, err := crypto.RecoverSigningPubkey(tx.Hash(), sig)
	if err != nil {
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	tx.From = pubkey.Address()
	////////////////////////////////

	t.Process.Go(nil, "HandleTxReceived", func(ctx context.Context) {
		t.HandleTxReceived(tx, peerConn)
	})
}

func (t *transport) servePostPrivateTx(w http.ResponseWriter, r *http.Request, peerConn *peerConn) {
	t.Infof(0, "incoming private tx")

	var encryptedTx prototree.EncryptedTx
	err := json.NewDecoder(r.Body).Decode(&encryptedTx)
	if err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	t.HandlePrivateTxReceived(encryptedTx, peerConn)
}

func (t *transport) NewPeerConn(ctx context.Context, dialAddr string) (swarm.PeerConn, error) {
	if t.ownURLs.Contains(dialAddr) || strings.HasPrefix(dialAddr, "localhost") {
		return nil, errors.WithStack(swarm.ErrPeerIsSelf)
	}
	return t.makePeerConn(nil, nil, dialAddr, types.ID{}, "", types.Address{}), nil
}

func (t *transport) ProvidersOfStateURI(ctx context.Context, stateURI string) (_ <-chan prototree.TreePeerConn, err error) {
	defer errors.AddStack(&err)

	providers, err := t.tryFetchProvidersFromAuthoritativeHost(stateURI)
	if err != nil {
		t.Warnf("could not fetch providers of state URI '%v' from authoritative host: %v", stateURI, err)
		return nil, err
	}

	u, err := url.Parse("http://" + stateURI)
	if err == nil {
		providers = append(providers, "http://"+u.Hostname())
	}

	ch := make(chan prototree.TreePeerConn)
	t.Process.Go(nil, "ProvidersOfStateURI", func(ctx context.Context) {
		defer close(ch)
		for _, providerURL := range providers {
			if t.ownURLs.Contains(providerURL) {
				continue
			}

			select {
			case ch <- t.makePeerConn(nil, nil, providerURL, types.ID{}, "", types.Address{}):
			case <-ctx.Done():
			}
		}
	})
	return ch, nil
}

func (t *transport) tryFetchProvidersFromAuthoritativeHost(stateURI string) ([]string, error) {
	parts := strings.Split(stateURI, "/")

	u, err := url.Parse("http://" + parts[0] + ":80?state_uri=" + stateURI)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "providers")

	ctx, cancel := context.WithTimeout(t.Ctx(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.Errorf("got status code %v %v: %v", resp.StatusCode, resp.Status, string(body))
	}

	var providers []string
	err = json.NewDecoder(resp.Body).Decode(&providers)
	if err != nil {
		return nil, err
	}
	return providers, nil
}

func (t *transport) ProvidersOfBlob(ctx context.Context, blobID blob.ID) (<-chan protoblob.BlobPeerConn, error) {
	return nil, errors.ErrUnimplemented
}

func (t *transport) AnnounceBlob(ctx context.Context, blobID blob.ID) error {
	return errors.ErrUnimplemented
}

var (
	ErrBadCookie = errors.New("bad cookie")
)

func (t *transport) ensureSessionIDCookieOnResponse(w http.ResponseWriter, r *http.Request) (types.ID, error) {
	sessionIDBytes, err := t.signedCookie(r, "sessionid")
	if err != nil {
		// t.Errorf("error reading signed sessionid cookie: %v", err)
		return t.setSessionIDCookieOnResponse(w)
	}
	return types.IDFromBytes(sessionIDBytes), nil
}

func (t *transport) setSessionIDCookieOnResponse(w http.ResponseWriter) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookieOnResponse(w, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *transport) setSessionIDCookieOnRequest(r *http.Request) (types.ID, error) {
	sessionID := types.RandomID()
	err := t.setSignedCookieOnRequest(r, "sessionid", sessionID[:])
	return sessionID, err
}

func (t *transport) setSignedCookieOnResponse(w http.ResponseWriter, name string, value []byte) error {
	w.Header().Del("Set-Cookie")

	cookie, err := t.makeSignedCookie(name, value)
	if err != nil {
		return err
	}
	http.SetCookie(w, cookie)
	return nil
}

func (t *transport) setSignedCookieOnRequest(r *http.Request, name string, value []byte) error {
	cookie, err := t.makeSignedCookie(name, value)
	if err != nil {
		return err
	}
	r.AddCookie(cookie)
	return nil
}

func (t *transport) makeSignedCookie(name string, value []byte) (*http.Cookie, error) {
	publicIdentity, err := t.keyStore.DefaultPublicIdentity()
	if err != nil {
		return nil, err
	}

	sig, err := t.keyStore.SignHash(publicIdentity.Address(), types.HashBytes(append(value, t.cookieSecret[:]...)))
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:    name,
		Value:   hex.EncodeToString(value) + ":" + hex.EncodeToString(sig),
		Expires: time.Now().AddDate(0, 0, 1),
		Path:    "/",
	}, nil
}

func (t *transport) signedCookie(r *http.Request, name string) ([]byte, error) {
	cookie, err := r.Cookie(name)
	if errors.Cause(err) == http.ErrNoCookie {
		return nil, errors.Err404
	} else if err != nil {
		return nil, err
	}
	parts := strings.Split(cookie.Value, ":")
	if len(parts) != 2 {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' has %v parts", name, len(parts))
	}

	value, err := hex.DecodeString(parts[0])
	if err != nil {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' bad hex value: %v", name, err)
	}

	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' bad hex signature: %v", name, err)
	}

	publicIdentity, err := t.keyStore.DefaultPublicIdentity()
	if err != nil {
		return nil, err
	}

	valid, err := t.keyStore.VerifySignature(publicIdentity.Address(), types.HashBytes(append(value, t.cookieSecret[:]...)), sig)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, errors.Wrapf(ErrBadCookie, "cookie '%v' has invalid signature (value: %0x)", name, value)
	}
	return value, nil
}

func (t *transport) makePeerConn(writer io.Writer, flusher http.Flusher, dialAddr string, sessionID types.ID, deviceUniqueID string, address types.Address) *peerConn {
	peer := &peerConn{t: t, sessionID: sessionID}
	peer.stream.Writer = writer
	peer.stream.Flusher = flusher

	if !address.IsZero() {
		peer.PeerEndpoint = t.peerStore.AddVerifiedCredentials(swarm.PeerDialInfo{TransportName, dialAddr}, deviceUniqueID, address, nil, nil)
	}
	peer.PeerEndpoint = t.peerStore.AddDialInfo(swarm.PeerDialInfo{TransportName, dialAddr}, deviceUniqueID)
	return peer
}

func (t *transport) doRequest(req *http.Request) (*http.Response, error) {
	altSvcHeader := makeAltSvcHeader(t.peerStore.AllDialInfos())
	req.Header.Set("Alt-Svc", altSvcHeader)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	t.storeAltSvcHeaderPeers(resp.Header)

	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		bs, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = errors.Wrapf(err, "error reading response body")
			err2 := errors.Errorf("http request errored: (%v) %v", resp.StatusCode, resp.Status)
			return resp, multierr.Append(err, err2)
		}
		return resp, errors.Errorf("http request errored: (%v) %v: %v", resp.StatusCode, resp.Status, string(bs))
	}
	return resp, nil
}
