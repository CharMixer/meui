package consents

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  aap "github.com/charmixer/aap/client"
  idp "github.com/charmixer/idp/client"

  bulky "github.com/charmixer/bulky/client"
)

func ShowConsents(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowConsents",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    aapClient := app.AapClientUsingAuthorizationCode(env, c)
    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    // Consents

    callUrl := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.consents.collection")
    status, responses, err := aap.ReadConsents(aapClient, callUrl, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == http.StatusForbidden {
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    if status != http.StatusOK {
      log.Debug("Failed to read consents")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var consents aap.ReadConsentsResponse
    status, restErr := bulky.Unmarshal(0, responses, &consents)
    if len(restErr) > 0 {
      for _,e := range restErr {
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == http.StatusForbidden {
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    if status != http.StatusOK {
      log.Debug("Failed to unmarshal ReadConsentsResponse")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    log.Debug(consents)

    // reference_id = access token subject.
    mapPublishers := make(map[string]string)
    mapClients := make(map[string]string)

    for _, consent := range consents {
      mapPublishers[consent.Publisher] = consent.Publisher
      mapClients[consent.Subscriber] = consent.Subscriber
    }

    // Read clients
    callUrl = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection")
    status, responses, err = idp.ReadClients(idpClient, callUrl, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == http.StatusForbidden {
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    if status != http.StatusOK {
      log.Debug("Failed to read clients")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var clients idp.ReadClientsResponse
    status, restErr = bulky.Unmarshal(0, responses, &clients)
    if len(restErr) > 0 {
      for _,e := range restErr {
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == http.StatusForbidden {
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    if status != http.StatusOK {
      log.Debug("Failed to unmarshal ReadClientsResponse")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    log.Debug(clients)

    c.HTML(http.StatusOK, "consents.html", gin.H{
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "title": "Consents",
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitConsents(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitConsents",
    })
  }
  return gin.HandlerFunc(fn)
}