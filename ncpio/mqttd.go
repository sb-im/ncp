package ncpio

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	lrucache "sb.im/ncp/cache"
	"sb.im/ncp/history"
	"sb.im/ncp/util"

	packets "github.com/eclipse/paho.golang/packets"
	paho "github.com/eclipse/paho.golang/paho"
	"github.com/sb-im/jsonrpc-lite"

	logger "log"
)

type Mqtt struct {
	Online  string
	Archive *history.Archive
	Client  *paho.Client
	Connect *paho.Connect
	Config  *MqttdConfig
	lru     *lrucache.LRUCache
	status  *NodeStatus
	cache   chan []byte
	I       <-chan []byte
	O       chan<- []byte
}

func NewMqtt(params string, i <-chan []byte, o chan<- []byte) *Mqtt {
	config, err := loadMqttConfigFromFile(params)
	if err != nil {
		logger.Println(err)
	}

	opt, err := url.Parse(config.Broker)
	if err != nil {
		logger.Println(err)
		return nil
	}
	logger.Printf("%+v\n", config)
	logger.Printf("%+v\n", config.Static)

	password, _ := opt.User.Password()
	status := &NodeStatus{
		Status: config.Static,
	}
	raw, _ := json.Marshal(status.SetOnline("neterror"))
	lru := lrucache.NewLRUCache(config.Rpc.LRU)
	cache := make(chan []byte, 128)

	// 2h
	//sessionExpiryInterval := uint32(7200)

	return &Mqtt{
		Online:  "online",
		lru:     &lru,
		Archive: history.New(128),

		I:      i,
		O:      o,
		cache:  cache,
		status: status,
		Config: config,
		Client: paho.NewClient(paho.ClientConfig{
			ClientID: fmt.Sprint(config.Client, config.ID),
			Router: paho.NewSingleHandlerRouter(func(p *paho.Publish) {
				if rpc := jsonrpc.ParseObject(p.Payload); rpc.Method == "history" {
					cache <- p.Payload

					// Only Record Jsonrpc Request
				} else if rpc.Type == jsonrpc.TypeRequest {
					// Same message filtering
					if data := lru.Get(rpc.ID.String()); data == "" {
						lru.Put(rpc.ID.String(), string(p.Payload))
						o <- p.Payload

						// jsonrpc is Idempotent
					} else if r := jsonrpc.ParseObject([]byte(data)); r.Type == jsonrpc.TypeSuccess || r.Type == jsonrpc.TypeErrors {
						cache <- []byte(data)
					}
				} else if rpc.Type == jsonrpc.TypeNotify {
					o <- p.Payload
				}
			}),
		}),
		Connect: paho.ConnectFromPacketConnect(&packets.Connect{
			WillProperties: &packets.Properties{},

			WillFlag:    true,
			WillMessage: raw,
			WillRetain:  true,
			WillTopic:   fmt.Sprintf(config.Status, config.ID),
			WillQOS:     1,
			Password:    []byte(password),
			Username:    opt.User.Username(),
			// https://stackoverflow.com/questions/65314401/cannot-connect-to-mosquitto-2-0-with-paho-library
			PasswordFlag: true,
			UsernameFlag: true,
			ClientID:     fmt.Sprintf(config.Client, config.ID),
			//CleanStart:  false,
			CleanStart: true,
			// interval 10s
			KeepAlive: 10,
			// TODO:
			Properties: &packets.Properties{
				// PayloadFormat indicates the format of the payload of the message
				// 0 is unspecified bytes
				// 1 is UTF8 encoded character data
				//PayloadFormat: 1,
				// MessageExpiry is the lifetime of the message in seconds
				//MessageExpiry *uint32
				//// ContentType is a UTF8 string describing the content of the message
				//// for example it could be a MIME type
				//ContentType string
				//// ResponseTopic is a UTF8 string indicating the topic name to which any
				//// response to this message should be sent
				//ResponseTopic string
				//// CorrelationData is binary data used to associate future response
				//// messages with the original request message
				//CorrelationData []byte
				//// SubscriptionIdentifier is an identifier of the subscription to which
				//// the Publish matched
				//SubscriptionIdentifier *uint32
				//// SessionExpiryInterval is the time in seconds after a client disconnects
				//// that the server should retain the session information (subscriptions etc)
				//SessionExpiryInterval: &sessionExpiryInterval,
				//// AssignedClientID is the server assigned client identifier in the case
				//// that a client connected without specifying a clientID the server
				//// generates one and returns it in the Connack
				//AssignedClientID string
				//// ServerKeepAlive allows the server to specify in the Connack packet
				//// the time in seconds to be used as the keep alive value
				//ServerKeepAlive *uint16
				//// AuthMethod is a UTF8 string containing the name of the authentication
				//// method to be used for extended authentication
				//AuthMethod string
				//// AuthData is binary data containing authentication data
				//AuthData []byte
				//// RequestProblemInfo is used by the Client to indicate to the server to
				//// include the Reason String and/or User Properties in case of failures
				//RequestProblemInfo *byte
				//// WillDelayInterval is the number of seconds the server waits after the
				//// point at which it would otherwise send the will message before sending
				//// it. The client reconnecting before that time expires causes the server
				//// to cancel sending the will
				//WillDelayInterval *uint32
				//// RequestResponseInfo is used by the Client to request the Server provide
				//// Response Information in the Connack
				//RequestResponseInfo *byte
				//// ResponseInfo is a UTF8 encoded string that can be used as the basis for
				//// createing a Response Topic. The way in which the Client creates a
				//// Response Topic from the Response Information is not defined. A common
				//// use of this is to pass a globally unique portion of the topic tree which
				//// is reserved for this Client for at least the lifetime of its Session. This
				//// often cannot just be a random name as both the requesting Client and the
				//// responding Client need to be authorized to use it. It is normal to use this
				//// as the root of a topic tree for a particular Client. For the Server to
				//// return this information, it normally needs to be correctly configured.
				//// Using this mechanism allows this configuration to be done once in the
				//// Server rather than in each Client
				//ResponseInfo string
				//// ServerReference is a UTF8 string indicating another server the client
				//// can use
				//ServerReference string
				//// ReasonString is a UTF8 string representing the reason associated with
				//// this response, intended to be human readable for diagnostic purposes
				//ReasonString string
				//// ReceiveMaximum is the maximum number of QOS1 & 2 messages allowed to be
				//// 'inflight' (not having received a PUBACK/PUBCOMP response for)
				//ReceiveMaximum *uint16
				//// TopicAliasMaximum is the highest value permitted as a Topic Alias
				//TopicAliasMaximum *uint16
				//// TopicAlias is used in place of the topic string to reduce the size of
				//// packets for repeated messages on a topic
				//TopicAlias *uint16
				//// MaximumQOS is the highest QOS level permitted for a Publish
				//MaximumQOS *byte
				//// RetainAvailable indicates whether the server supports messages with the
				//// retain flag set
				//RetainAvailable *byte
				//// User is a map of user provided properties
				//User map[string]string
				//// MaximumPacketSize allows the client or server to specify the maximum packet
				//// size in bytes that they support
				//MaximumPacketSize *uint32
				//// WildcardSubAvailable indicates whether wildcard subscriptions are permitted
				//WildcardSubAvailable *byte
				//// SubIDAvailable indicates whether subscription identifiers are supported
				//SubIDAvailable *byte
				//// SharedSubAvailable indicates whether shared subscriptions are supported
				//SharedSubAvailable *byte
			},
		}),
	}
}

func (t *Mqtt) Run(ctx context.Context) {
	//t.Client.SetDebugLogger(logger.New(os.Stdout, "[DEBUG]: ", logger.LstdFlags | logger.Lshortfile))
	//t.Client.SetErrorLogger(logger.New(os.Stdout, "[ERROR]: ", logger.LstdFlags | logger.Lshortfile))

	opt, err := url.Parse(t.Config.Broker)
	if err != nil {
		logger.Println(err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			logger.Println("MQTT Try Connect")
			if conn, err := net.Dial("tcp", opt.Hostname()+":"+opt.Port()); err != nil {
				logger.Println(err)
			} else {
				logger.Println("MQTT TCP Connected")
				t.Client.Conn = conn
				t.doRun(ctx)
				conn.Close()
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (t *Mqtt) doRun(parent context.Context) {
	pinger := NewPingHandler(t.Client, fmt.Sprintf(t.Config.Network, t.Config.ID))
	//pinger.SetDebug(logger.New(os.Stdout, "[Pinger]: ", logger.LstdFlags | logger.Lshortfile))
	t.Client.PingHandler = pinger

	ctx, cancel := context.WithCancel(parent)
	t.Client.OnServerDisconnect = func(p *paho.Disconnect) {
		logger.Println("OnDisconnect: ", p)
		cancel()
	}

	t.Client.OnClientError = func(err error) {
		logger.Println("OnClientError: ", err)
		cancel()
	}

	defer logger.Println("MQTT Close")
	if res, err := t.Client.Connect(ctx, t.Connect); err != nil {
		if res != nil {
			logger.Printf("%+v\n", res)
		}
		logger.Println("MQTT Connect failure: ", err)
		return
	}
	logger.Println("MQTT Connected")

	if res, err := t.Client.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			fmt.Sprintf(t.Config.Rpc.O, t.Config.ID): {
				QoS: t.Config.Rpc.QoS,
				//RetainHandling    byte
				//NoLocal           bool
				//RetainAsPublished bool
			},
		},
	}); err != nil {
		if res != nil {
			logger.Printf("%+v\n", res)
		}
		logger.Println(err)
		return
	}

	defer t.setStatus("offline")
	t.setStatus(t.Online)

	for {
		select {
		case raw := <-t.cache:
			if err := t.send(ctx, raw); err != nil {
				t.cache <- raw
				return
			}
		case raw := <-t.I:
			if err := t.send(ctx, raw); err != nil {
				t.cache <- raw
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (t *Mqtt) send(ctx context.Context, raw []byte) error {
	if rpc, err := jsonrpc.Parse(raw); err == nil && (rpc.Type == jsonrpc.TypeSuccess || rpc.Type == jsonrpc.TypeErrors) {
		//fmt.Println("[RES]: ", string(raw))
		// {"jsonrpc":"2.0","result":"ok","id":"test.0-1607482556696-0"}
		// {"jsonrpc":"2.0","error":{"code":-32601,"message":"Method not found"},"id":"test.0-99991607483766.0"}

		// Idempotent Record result
		t.lru.Put(rpc.ID.String(), string(raw))

		res, err := t.Client.Publish(ctx, &paho.Publish{
			Payload: raw,
			Topic:   fmt.Sprintf(t.Config.Rpc.I, t.Config.ID),
			QoS:     t.Config.Rpc.QoS,
		})

		if err != nil {
			if res != nil {
				logger.Printf("%+v\n", res)
			}
			logger.Println(err)
			return err
		}
	} else if err == nil && (rpc.Type == jsonrpc.TypeRequest || rpc.Type == jsonrpc.TypeNotify) {
		//fmt.Println("[REQ]: ", string(raw))
		// JSON-RPC Request Ignore

		// {"jsonrpc":"2.0","method":"test","params":[]}
		// {"jsonrpc":"2.0","id":"test.0-1553321035000","method":"test","params":[]}

		// {"jsonrpc":"2.0","method":"ncp_offline"}

		if rpc.Method == "history" {
			type Params struct {
				Topic string `json:"topic"`
				Time  string `json:"time"`
			}
			raw_params, _ := rpc.Params.MarshalJSON()
			params := &Params{}
			err := json.Unmarshal(raw_params, params)

			if err != nil {
				rpc.Errors.Message = "Params Error"
			} else {
				historys := t.Archive.GetLatestHistorys(strings.Split(params.Topic, "/")[1], params.Time)
				results := make(map[string]json.RawMessage, len(historys))
				for _, h := range historys {
					results[strconv.FormatInt(h.Time.Unix(), 10)] = json.RawMessage(h.Data)
				}
				d, _ := json.Marshal(results)
				dd := json.RawMessage(d)
				rpc.Result = &dd
			}
			rpc.Method = ""
			rpc.Params = nil
			data, _ := rpc.ToJSON()
			logger.Printf("%s\n", data)
			t.cache <- data
		}

		if rpc.Method == "ncp_online" {
			t.Online = "online"
		}
		if rpc.Method == "ncp_offline" {
			t.Online = "offline"
		}
		t.setStatus(t.Online)

	} else {
		//fmt.Println("[Tran]: ", string(raw))

		for key, data := range util.DetachTran(raw) {
			if !t.Archive.FilterAdd(key, data) {
				continue
			}
			opt, ok := t.Config.Trans[key]
			if !ok {
				// TODO: 'opt' use Default
			}
			if res, err := t.Client.Publish(ctx, &paho.Publish{
				Payload: data,
				Topic:   fmt.Sprintf(t.Config.Gtran.Prefix, t.Config.ID, key),
				QoS:     opt.QoS,
				Retain:  opt.Retain,
				//Properties *Properties
				//PacketID   uint16
				//Duplicate  bool

			}); err != nil {
				if res != nil {
					logger.Printf("%+v\n", res)
				}
				logger.Println(err)
				return err
			}
		}
	}
	return nil
}

func (t *Mqtt) setStatus(str string) error {
	logger.Println("Set Status: ", str)
	raw, err := json.Marshal(t.status.SetOnline(str))
	if err != nil {
		logger.Println(err)
		return err
	} else {
		if res, err := t.Client.Publish(context.Background(), &paho.Publish{
			Payload: raw,
			Topic:   fmt.Sprintf(t.Config.Status, t.Config.ID),
			QoS:     1,
			Retain:  true,
		}); err != nil {
			if res != nil {
				logger.Printf("%+v\n", res)
			}
			logger.Println(err)
			return err
		}
	}
	return nil
}
