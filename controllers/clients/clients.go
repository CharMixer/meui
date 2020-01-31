package clients

import (
  "net/url"
  "net/http"
  "sort"
  "fmt"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"

  bulky "github.com/charmixer/bulky/client"
)

type ClientTemplate struct {
  Id string
  Name string
  Description string
  Secret string
  ClientEditUrl string
  GrantsUrl string
  SubscriptionsUrl string
  DeleteUrl string
}

func ShowClients(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowClients",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    status, responses, err := idp.ReadClients(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection"), nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status != 200 {
      log.Debug(fmt.Sprintf("Failed to get OK response, got %d", status))
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    clientEditUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.clients"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    grantsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.access.grant"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    subscriptionsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.subscriptions.collection"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    deleteUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.clients.delete"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var uiCreatedClients []ClientTemplate

    var clients idp.ReadClientsResponse
    status, _ = bulky.Unmarshal(0, responses, &clients)
    if status == 200 {

      for _, client := range clients {

        _clientEditUrl := *clientEditUrl
        q := _clientEditUrl.Query()
        q.Add("id", client.Id)
        _clientEditUrl.RawQuery = q.Encode()

        _grantsUrl := *grantsUrl
        q = _grantsUrl.Query()
        q.Add("receiver", client.Id)
        _grantsUrl.RawQuery = q.Encode()

        _subscriptionsUrl := *subscriptionsUrl
        q = _subscriptionsUrl.Query()
        q.Add("receiver", client.Id)
        _subscriptionsUrl.RawQuery = q.Encode()

        _deleteUrl := *deleteUrl
        q = _deleteUrl.Query()
        q.Add("id", client.Id)
        _deleteUrl.RawQuery = q.Encode()

        uiClient := ClientTemplate{
          Id:        client.Id,
          Name:      client.Name,
          Description: client.Description,
          Secret: client.Secret,
          ClientEditUrl: _clientEditUrl.String(),
          GrantsUrl: _grantsUrl.String(),
          SubscriptionsUrl: _subscriptionsUrl.String(),
          DeleteUrl: _deleteUrl.String(),
        }
        uiCreatedClients = append(uiCreatedClients, uiClient)

      }

    }

    sort.Slice(uiCreatedClients, func(i, j int) bool {
      return uiCreatedClients[i].Name > uiCreatedClients[j].Name
    })

    c.HTML(http.StatusOK, "clients.html", gin.H{
      "title": "Clients",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "created": uiCreatedClients,
    })
  }
  return gin.HandlerFunc(fn)
}
