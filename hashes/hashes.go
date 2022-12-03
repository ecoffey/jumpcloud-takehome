package hashes

import (
	"crypto/sha512"
	"encoding/base64"
	"log"
	"time"
)

type HashCmdReserveId struct {
	Plaintext string   // the string to hash and encode
	Resp      chan int // the channel to send the reserved currentId back to
}

// This command is only used internally to this package
type hashCmdStore struct {
	id   int    // the id to store at
	hash string // the hash to store
}

type HashCmdRetrieve struct {
	Id   int         // the Id to retrieve
	Resp chan string // the channel to send the hash to
}

type HashCmdGracefulShutdown struct{}

type hashStore struct {
	cmds               chan interface{} // the command queue used to send commands to the store
	currentId          int              // the current id to be sent back to HashCmdReserveId.Resp
	idToHash           map[int]string   // the mapping from id to hash for retrieval
	hashDelay          time.Duration    // delay to wait during HashCmdReserveId before issuing a HashCmdStore
	inFlight           int              // the number of hashes waiting to be added to the store
	acceptingNewHashes bool             // if true the HashCmdReserveId will return currentId and schedule a store
	shutdown           chan int         // channel to signal when inFlight == 0 and no new hashes are being accepted
}

// StartHashLoop builds and returns a channel of empty interface, where
// the intention is to feed that channel the HashCmd* types, and begins
// consuming from it in a go routine.
//
// shutdown: a channel to signal that we have been asked to shut down and
// are done processing in-flight hashes.
//
// hashDelay: a Duration to wait before storing the hash in the hash store.
// if > 0, then the hash is stored after hashDelay time via the
// hashCmdStore command.
//
// if == 0, then the hash is stored immediately during the processing of
// the HashCmdReserveId command. This is useful for testing, so that tests
// do not have to specify time.Sleep() calls.
func StartHashLoop(shutdown chan int, hashDelay time.Duration) chan<- interface{} {

	s := hashStore{
		// specify a buffered channel, so that we can concurrently
		// process requests
		cmds:               make(chan interface{}, 100),
		currentId:          1,
		idToHash:           make(map[int]string),
		hashDelay:          hashDelay,
		inFlight:           0,
		acceptingNewHashes: true,
		shutdown:           shutdown,
	}

	go func() {
		for cmd := range s.cmds {
			switch cmd.(type) {
			case HashCmdReserveId:
				s.processReserveCmd(cmd.(HashCmdReserveId))
			case hashCmdStore:
				s.processStoreCmd(cmd.(hashCmdStore))
			case HashCmdRetrieve:
				s.processRetrieveCmd(cmd.(HashCmdRetrieve))
			case HashCmdGracefulShutdown:
				s.processGracefulShutdownCmd(cmd.(HashCmdGracefulShutdown))
			default:
				log.Fatalln("unknown command type")
			}
		}
	}()

	return s.cmds
}

func (s *hashStore) processReserveCmd(cmd HashCmdReserveId) {
	if s.acceptingNewHashes {
		id := s.currentId
		cmd.Resp <- id
		s.currentId += 1
		s.inFlight++
		if s.hashDelay > 0 {
			go func() {
				<-time.Tick(s.hashDelay)
				s.cmds <- hashCmdStore{
					id:   id,
					hash: hashEncode(cmd.Plaintext),
				}
			}()
		} else {
			s.processStoreCmd(hashCmdStore{
				id:   id,
				hash: hashEncode(cmd.Plaintext),
			})
		}
	} else {
		cmd.Resp <- -1
	}
}

func (s *hashStore) processStoreCmd(cmd hashCmdStore) {
	s.idToHash[cmd.id] = cmd.hash
	s.inFlight--
	if !s.acceptingNewHashes && s.inFlight == 0 {
		// if this was the last hash we were waiting for, then we're
		// done and can signal the shutdown channel.
		s.shutdown <- 1
	}
}

func (s *hashStore) processRetrieveCmd(cmd HashCmdRetrieve) {
	cmd.Resp <- s.idToHash[cmd.Id]
}

func (s *hashStore) processGracefulShutdownCmd(cmd HashCmdGracefulShutdown) {
	s.acceptingNewHashes = false
	if s.inFlight == 0 {
		// If we're asked to shut down with nothing in flight, then
		// we can safely & immediately signal shutdown
		s.shutdown <- 1
	}
}

func hashEncode(plaintext string) string {
	// opted for this pattern instead of sha512.Sum512
	// so that we don't have to do a slice copy to
	// convert from [64]byte to []byte for passing into
	// EncodeToString

	hasher := sha512.New()
	hasher.Write([]byte(plaintext))
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil))
}
