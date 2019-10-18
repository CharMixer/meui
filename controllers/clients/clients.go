package clients

import (
  "net/url"
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  bulky "github.com/charmixer/bulky/client"
)

type ClientTemplate struct {
  Id string
  GrantsUrl string
}

func ShowClients(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowClients",
    })

    identity := app.RequireIdentity(c)
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
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var uiCreatedClients []ClientTemplate

    var clients idp.ReadClientsResponse
    status, _ = bulky.Unmarshal(0, responses, &clients)
    if status == 200 {

      for _, client := range clients {

        grantsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.access.grant"))
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }
        q := grantsUrl.Query()
        q.Add("receiver", client.Id)
        grantsUrl.RawQuery = q.Encode()

        uiClient := ClientTemplate{
          Id:        client.Id,
          GrantsUrl: grantsUrl.String(),
        }
        uiCreatedClients = append(uiCreatedClients, uiClient)

      }

    }

    c.HTML(http.StatusOK, "clients.html", gin.H{
      "title": "Clients",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "created": uiCreatedClients,
    })
  }
  return gin.HandlerFunc(fn)
}