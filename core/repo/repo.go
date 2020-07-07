package repo

import (
	"os"
	"os/user"

	"github.com/aloknerurkar/gopcp/core"
	"github.com/aloknerurkar/gopcp/core/repo/handlers/boltStore"
	"github.com/aloknerurkar/gopcp/core/repo/handlers/osFilestore"
	logger "github.com/ipfs/go-log"
	crypto "github.com/libp2p/go-libp2p-crypto"
)

var log = logger.Logger("Core::Repo")

const repoRoot = "/.gopcp"
const dbRoot = "/db"

type repo struct {
	privKey   crypto.PrivKey
	pubKey    crypto.PubKey
	fileStore core.Store
	kvStore   core.Store
}

// InitializeRepo initializes the Repo if it hasnt been already.
// It will create the repo in users home directory in the
// .gopcp folder
func InitializeRepo() (core.Repo, error) {

	usr, err := user.Current()
	if err != nil {
		log.Errorf("Current user not found Err:%s", err.Error())
		return nil, err
	}

	if _, err = os.Stat(usr.HomeDir + repoRoot); err != nil {
		if !os.IsNotExist(err) {
			log.Errorf("Unable to stat Repo root dir Err:%s", err.Error())
			return nil, err
		}

		// If repo doesn't exist, create it
		err = os.Mkdir(usr.HomeDir+repoRoot, os.ModePerm)
		if err != nil {
			log.Errorf("Failed creating repo Err:%s", err.Error())
			return nil, err
		}

		err = os.Mkdir(usr.HomeDir+repoRoot+dbRoot, os.ModePerm)
		if err != nil {
			log.Errorf("Failed creating DB root dir Err:%s", err.Error())
			return nil, err
		}
	}

	cfg := map[string]interface{}{
		"root":   usr.HomeDir + repoRoot,
		"dbName": "files",
	}
	fsRepo, err := osFilestore.NewOSFileStore(cfg)
	if err != nil {
		log.Errorf("Failed creating OS Filestore Err:%s", err.Error())
		return nil, err
	}

	cfg = map[string]interface{}{
		"root":   usr.HomeDir + repoRoot + dbRoot,
		"dbName": "kvRepo",
	}
	kvRepo, err := boltStore.NewBoltStore(cfg)
	if err != nil {
		log.Errorf("Failed creating KV repo Err:%s", err.Error())
		return nil, err
	}

	log.Infof("Done initializing Repo at %s", usr.HomeDir+repoRoot)

	r := &repo{
		fileStore: fsRepo,
		kvStore:   kvRepo,
	}

	err = r.initializeIdentity()
	if err != nil {
		return nil, err
	}

	return r, nil
}

type keyFile struct {
	name string
	*os.File
}

func (k *keyFile) GetId() string {
	return k.name
}

func (k *keyFile) GetTable() string {
	return "identity"
}

func (k *keyFile) SetFp(f *os.File) {
	k.File = f
}

func (r *repo) readKeyFromFile(f *keyFile, unmarshal func([]byte) error) error {

	err := r.fileStore.Read(f)
	if err != nil {
		log.Errorf("Unable to read file Err:%s", err.Error())
		return err
	}

	st, err := f.Stat()
	if err != nil {
		log.Errorf("Failed to stat file %s Err:%s", f.Name(), err.Error())
		return err
	}

	keyBuf := make([]byte, st.Size())
	_, err = f.Read(keyBuf)
	if err != nil {
		log.Errorf("Failed to read from file %s Err:%s", f.Name(), err.Error())
		return err
	}

	return unmarshal(keyBuf)
}

func (r *repo) writeKeyToFile(f *keyFile, marshal func() ([]byte, error)) error {

	err := r.fileStore.Create(f)
	if err != nil {
		log.Errorf("Failed creating file in repo Err:%s", err.Error())
		return err
	}
	defer f.Close()

	buf, err := marshal()
	if err != nil {
		log.Errorf("Failed getting bytes for file %s Err:%s", f.Name(), err.Error())
		return err
	}

	_, err = f.Write(buf)
	if err != nil {
		log.Errorf("Failed writing key to file %s Err:%s", f.Name(), err.Error())
		return err
	}

	return nil
}

func (r *repo) initializeIdentity() error {
	// If identity is not initialized, initialize it
	privKeyFile := &keyFile{name: "private_key"}
	pubKeyFile := &keyFile{name: "public_key"}
	err := r.fileStore.Read(privKeyFile)
	if err != nil {
		log.Infof("No existing key file found. Generating one...")
		priv, pub, err := crypto.GenerateKeyPair(crypto.Secp256k1, 256)
		if err != nil {
			log.Errorf("Failed initializing key pair Err:%s", err.Error())
			return err
		}

		err = r.writeKeyToFile(privKeyFile, func() ([]byte, error) {
			return crypto.MarshalPrivateKey(priv)
		})
		if err != nil {
			return err
		}

		err = r.writeKeyToFile(pubKeyFile, func() ([]byte, error) {
			return crypto.MarshalPublicKey(pub)
		})
		if err != nil {
			return err
		}
		// No need to read from file again
		r.privKey = priv
		r.pubKey = pub
		log.Infof("Initialized new public and private key pair")
		return nil
	}

	var privKey crypto.PrivKey
	err = r.readKeyFromFile(privKeyFile, func(buf []byte) error {
		privKey, err = crypto.UnmarshalPrivateKey(buf)
		return err
	})

	var pubKey crypto.PubKey
	err = r.readKeyFromFile(pubKeyFile, func(buf []byte) error {
		pubKey, err = crypto.UnmarshalPublicKey(buf)
		return err
	})
	r.privKey = privKey
	r.pubKey = pubKey

	log.Infof("Loaded existing public and private key pair")
	return nil
}

func (r *repo) PrivKey() crypto.PrivKey {
	return r.privKey
}

func (r *repo) PubKey() crypto.PubKey {
	return r.pubKey
}

func (r *repo) FsRepo() core.Store {
	return r.fileStore
}

func (r *repo) KVRepo() core.Store {
	return r.kvStore
}

func (r *repo) Conf() core.Config {
	return &tempConf{}
}

type tempConf struct{}

func (t *tempConf) Get(field string, buf interface{}) bool {
	if field == "listenAddr" {
		if strP, ok := buf.(*string); ok {
			*strP = "0.0.0.0"
			return true
		}
	}

	if field == "p2pPort" {
		if intP, ok := buf.(*int); ok {
			*intP = 10000
			return true
		}
	}

	if field == "gatewayRpcPort" {
		if intP, ok := buf.(*int); ok {
			*intP = 10001
			return true
		}
	}

	if field == "gatewayHttpPort" {
		if intP, ok := buf.(*int); ok {
			*intP = 10002
			return true
		}
	}

	return false
}
