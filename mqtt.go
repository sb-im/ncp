package main

import (
  "os"
  "log"
  "time"
  "net/url"

  mqtt "github.com/eclipse/paho.mqtt.golang"
)

func setUri(uri *url.URL) *mqtt.ClientOptions {
  opts := mqtt.NewClientOptions()
  //opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
  opts.AddBroker("tcp://" + uri.Host)
  opts.SetUsername(uri.User.Username())
  password, _ := uri.User.Password()
  opts.SetPassword(password)
  return opts
}

type mqttProxy struct {
  client mqtt.Client
  id string
  ch_rpc_send chan string
  ch_rpc_recv chan string
}

func (this *mqttProxy) Connect(clientId string, uri *url.URL, willTopic string) mqtt.Client {
  opts := setUri(uri)
  opts.SetWill(willTopic, "2", 2, true)
  logger := log.New(os.Stdout, "[Mqtt] ", log.LstdFlags)

  opts.SetClientID(clientId)

  // interval 2s
  opts.SetKeepAlive(2 * time.Second)
  opts.SetResumeSubs(true)

  // Lost callback
  opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
    logger.Println("Lost Connect")
  })

  // Connect && Reconnect callback
  opts.SetOnConnectHandler(func (client mqtt.Client) {
    logger.Println("New Connect")
    clientOptionsReader := client.OptionsReader()
    mqttSetOnline(client, clientOptionsReader.WillTopic(), "online")
    go mqttRecv(client, "nodes/" + this.id + "/rpc/send", 2, this.ch_rpc_recv)
  })

  client := mqtt.NewClient(opts)
  token := client.Connect()
  for !token.WaitTimeout(3 * time.Second) {
  }
  if err := token.Error(); err != nil {
    logger.Fatal(err)
  }
  this.client = client
  return client
}
