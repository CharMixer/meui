package subscriptions

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"

  bulky "github.com/charmixer/bulky/client"

  aap "github.com/charmixer/aap/client"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  "github.com/charmixer/meui/app"
  f "github.com/go-playground/form"
  "fmt"
)

type formInput struct {
  Publishings   []struct{
    Scope          string `form:"" validate:""`
    Subscribed     bool
  }
}

func ShowSubscriptions(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowSubscriptions",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    publisher, publisherExists := c.GetQuery("publisher")
    receiver, receiverExists := c.GetQuery("receiver")
    if !receiverExists {
      receiver = identity.Id
    }

    var url string
    var responses []bulky.Response
    var err error
    var restErr []bulky.ErrorResponse

    aapClient := app.AapClientUsingAuthorizationCode(env, c)
    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    var readPublishesResponse aap.ReadPublishesResponse
    if publisherExists {
      url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.publishes")
      status, responses, err := aap.ReadPublishes(aapClient, url, []aap.ReadPublishesRequest{ {Publisher: publisher} })
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
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      _, restErr = bulky.Unmarshal(0, responses, &readPublishesResponse)
      if len(restErr) > 0 {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }

        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }
    }

    url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.subscriptions.collection")
    status, responses, err := aap.ReadSubscriptions(aapClient, url, []aap.ReadSubscriptionsRequest{ {Subscriber: receiver} })
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
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var subscriptions aap.ReadSubscriptionsResponse
    _, restErr = bulky.Unmarshal(0, responses, &subscriptions)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var hasSubscribedMap = make(map[string]bool, len(subscriptions))
    for _,s := range subscriptions {
      hasSubscribedMap[s.Scope] = true
    }

    url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection")
    status, responses, err = idp.ReadResourceServers(idpClient, url, nil)
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
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var resourceservers idp.ReadResourceServersResponse
    _, restErr = bulky.Unmarshal(0, responses, &resourceservers)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    c.HTML(200, "subscriptions.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "title": "Subscriptions for " + receiver,
      "hasSubscribedMap": hasSubscribedMap,
      "publishes": readPublishesResponse,
      "resourceservers": resourceservers,
      "publisher": publisher,
      "receiver": receiver,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitSubscriptions(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitSubscriptions",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    aapClient := app.AapClientUsingAuthorizationCode(env, c)

    publisher, publisherExists := c.GetQuery("publisher")
    receiver, receiverExists := c.GetQuery("receiver")

    if !publisherExists || !receiverExists  {
      log.WithFields(logrus.Fields{
        "publisher": publisher,
        "receiver": receiver,
      }).Debug("publisher and receiver must exists")
      c.AbortWithStatus(http.StatusBadRequest)
      return
    }

    var form formInput
    c.Request.ParseForm()

    decoder := f.NewDecoder()

    // must pass a pointer
    err := decoder.Decode(&form, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(404)
      return
    }

    var createSubscriptionsRequests []aap.CreateSubscriptionsRequest
    var deleteSubscriptionsRequests []aap.DeleteSubscriptionsRequest
    for _,publishing := range form.Publishings {
      if publishing.Subscribed {
        createSubscriptionsRequests = append(createSubscriptionsRequests, aap.CreateSubscriptionsRequest{
          Subscriber: receiver,
          Publisher: publisher,
          Scope: publishing.Scope,
        })
        continue;
      }

      // deny by default
      deleteSubscriptionsRequests = append(deleteSubscriptionsRequests, aap.DeleteSubscriptionsRequest{
        Subscriber: receiver,
        Publisher: publisher,
        Scope: publishing.Scope,
      })
    }

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.subscriptions")

    status, responses, err := aap.CreateSubscriptions(aapClient, url, createSubscriptionsRequests)
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
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    var createSubscriptions aap.CreateSubscriptionsResponse
    _, restErr := bulky.Unmarshal(0, responses, &createSubscriptions)
    if restErr != nil {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    log.Debug(createSubscriptions)

    c.Redirect(http.StatusFound, fmt.Sprintf("/subscriptions?receiver=%s&publisher=%s", receiver, publisher))
    c.Abort()
    return
  }
  return gin.HandlerFunc(fn)
}
