package sda_test

import (
	"github.com/dedis/cothority/lib/dbg"
	"github.com/dedis/cothority/lib/network"
	"github.com/dedis/cothority/lib/sda"
	"testing"
	"time"
)

type DummyProtocol struct {
	*sda.TreeNodeInstance
	link   chan bool
	config DummyConfig
}

type DummyConfig struct {
	A    int
	Send bool
}

type DummyMsg struct {
	A int
}

func init() {
	network.RegisterMessageType(DummyMsg{})
}

func NewDummyProtocol(tni *sda.TreeNodeInstance, conf DummyConfig, link chan bool) *DummyProtocol {
	return &DummyProtocol{tni, link, conf}
}

func (dm *DummyProtocol) Start() error {
	dm.link <- true
	if dm.config.Send {
		if err := dm.SendTo(dm.TreeNode(), &DummyMsg{}); err != nil {
		}
	}
	return nil
}

func (dm *DummyProtocol) DispatchMsg(msg *sda.Data) {
	dm.link <- true
}

// legcy reasons
func (dm *DummyProtocol) Dispatch() error {
	return nil
}

type DummyService struct {
	c        sda.Context
	path     string
	link     chan bool
	fakeTree *sda.Tree
	firstTni *sda.TreeNodeInstance
	Config   DummyConfig
}

func (ds *DummyService) ProcessRequest(e *network.Entity, r *sda.Request) {
	if r.Type != "NewDummy" {
		ds.link <- false
		return
	}
	if ds.firstTni == nil {
		ds.firstTni = ds.c.NewTreeNodeInstance(ds.fakeTree, ds.fakeTree.Root)
	}

	dp := NewDummyProtocol(ds.firstTni, ds.Config, ds.link)

	if err := ds.c.RegisterProtocolInstance(dp); err != nil {
		ds.link <- false
		return
	}
	dp.Start()
}

func (ds *DummyService) NewProtocol(tn *sda.TreeNodeInstance, conf *sda.GenericConfig) (sda.ProtocolInstance, error) {
	dummyConf := conf.Data.(DummyConfig)
	dummyConf.A++
	dp := NewDummyProtocol(tn, dummyConf, ds.link)
	return dp, nil
}

func TestServiceNew(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		ds.link <- true
		return ds
	})
	go func() {
		h := sda.NewLocalHost(2000)
		h.Close()
	}()

	waitOrFatal(ds.link, t)
}

func TestServiceProcessRequest(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		return ds
	})
	host := sda.NewLocalHost(2000)
	host.Listen()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// Send a request to the service
	re := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Type:    "wrongType",
	}
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link
	select {
	case v := <-ds.link:
		if v {
			t.Fatal("was expecting false !")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Too late")
	}
}

// Test if a request that makes the service create a new protocol works
func TestServiceRequestNewProtocol(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		ds.c = c
		ds.path = path
		return ds
	})
	host := sda.NewLocalHost(2000)
	host.Listen()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// create the entityList and tree
	el := sda.NewEntityList([]*network.Entity{host.Entity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds.fakeTree = tree

	// Send a request to the service
	re := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Type:    "NewDummy",
	}
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	waitOrFatalValue(ds.link, true, t)

	// Now RESEND the value so we instantiate using the SAME TREENODE
	dbg.Lvl1("Sending request AGAIN to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the
	// NOW expect false
	waitOrFatalValue(ds.link, false, t)
}

func TestServiceProtocolProcessMessage(t *testing.T) {
	defer dbg.AfterTest(t)
	dbg.TestOutput(testing.Verbose(), 4)
	ds := &DummyService{
		link: make(chan bool),
	}
	sda.RegisterNewService("DummyService", func(c sda.Context, path string) sda.Service {
		if ds.c != nil {
			// the client does not need a Service
			return &DummyService{link: make(chan bool)}
		}
		ds.c = c
		ds.path = path
		ds.Config = DummyConfig{
			Send: true,
		}
		return ds
	})
	host := sda.NewLocalHost(2000)
	host.ListenAndBind()
	host.StartProcessMessages()
	dbg.Lvl1("Host created and listening")
	defer host.Close()
	// create the entityList and tree
	el := sda.NewEntityList([]*network.Entity{host.Entity})
	tree := el.GenerateBinaryTree()
	// give it to the service
	ds.fakeTree = tree

	// Send a request to the service
	re := &sda.Request{
		Service: sda.ServiceFactory.ServiceID("DummyService"),
		Type:    "NewDummy",
	}
	// fake a client
	h2 := sda.NewLocalHost(2010)
	defer h2.Close()
	dbg.Lvl1("Client connecting to host")
	if _, err := h2.Connect(host.Entity); err != nil {
		t.Fatal(err)
	}
	dbg.Lvl1("Sending request to service...")
	if err := h2.SendRaw(host.Entity, re); err != nil {
		t.Fatal(err)
	}
	// wait for the link from the protocol
	waitOrFatalValue(ds.link, true, t)

	// now wait for the same link as the protocol should have sent a message to
	// himself !
	waitOrFatalValue(ds.link, true, t)
}

func waitOrFatalValue(ch chan bool, v bool, t *testing.T) {
	select {
	case b := <-ch:
		if v != b {
			t.Fatal("Wrong value returned on channel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Waited too long")
	}

}
func waitOrFatal(ch chan bool, t *testing.T) {
	select {
	case _ = <-ch:
		return
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Waited too long")
	}
}
