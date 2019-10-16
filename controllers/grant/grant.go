package grant

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  "golang.org/x/oauth2"
  oidc "github.com/coreos/go-oidc"

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
  Publisher     string
  Receiver       string
  Grants        []struct{
    Scope          string
    Enabled        bool
    StartDate      string
    EndDate        string
  }
}

func ShowGrants(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowAccess",
    })

    session := sessions.Default(c)

    identity := app.RequireIdentity(c)
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

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "grants.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)
    idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    var url string
    var responses []bulky.Response
    var err error
    var restErr []bulky.ErrorResponse

    // fetch publishes
    var publishes aap.ReadPublishesResponse
    var grantPublishes []aap.Publish
    var mayGrantPublishes []aap.Publish
    if publisherExists {
      url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.publishes")
      _, responses, err = aap.ReadPublishes(aapClient, url, []aap.ReadPublishesRequest{
        {Publisher: publisher},
      })

      if err != nil {
        c.AbortWithStatus(404)
        log.Debug(err.Error())
        return
      }

      _, restErr = bulky.Unmarshal(0, responses, &publishes)
      if len(restErr) > 0 {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }

        c.AbortWithStatus(404)
        return
      }

      for _,p := range publishes {
        if len(p.MayGrantScopes) > 0 {
          mayGrantPublishes = append(mayGrantPublishes, p)
          continue
        }

        grantPublishes = append(grantPublishes, p)
      }
    }

    // fetch grants

    url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.grants")
    _, responses, err = aap.ReadGrants(aapClient, url, []aap.ReadGrantsRequest{
      { Identity: receiver, Publisher: publisher},
    })

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    var grants aap.ReadGrantsResponse
    _, restErr = bulky.Unmarshal(0, responses, &grants)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    var hasGrantsMap = make(map[string]bool, len(grants))
    for _,g := range grants {
      hasGrantsMap[g.Scope] = true
    }

    // fetch resourceservers

    url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection")
    _, responses, err = idp.ReadResourceServers(idpClient, url, nil)

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    var resourceservers idp.ReadResourceServersResponse
    _, restErr = bulky.Unmarshal(0, responses, &resourceservers)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    c.HTML(200, "grants.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },

      "title": "Grants",
      "hasGrantsMap": hasGrantsMap,
      "grantPublishes": grantPublishes,
      "mayGrantPublishes": mayGrantPublishes,
      "resourceservers": resourceservers,
      "publisher": publisher,
      "receiver": receiver,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitGrants(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowAccess",
    })

    session := sessions.Default(c)

    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "grants.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    publisher, publisherExists := c.GetQuery("publisher")
    receiver, receiverExists := c.GetQuery("receiver")

    if !publisherExists || !receiverExists  {
      log.WithFields(logrus.Fields{
        "publisher": publisher,
        "receiver": receiver,
      }).Debug("publisher and receiver must exists")
      c.AbortWithStatus(404)
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

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)

    var createGrantsRequests []aap.CreateGrantsRequest
    var deleteGrantsRequests []aap.DeleteGrantsRequest
    for _,grant := range form.Grants {
      if grant.Enabled {
        createGrantsRequests = append(createGrantsRequests, aap.CreateGrantsRequest{
          Identity: receiver,
          Scope: grant.Scope,
          Publisher: publisher,
        })
        continue;
      }

      // deny by default
      deleteGrantsRequests = append(deleteGrantsRequests, aap.DeleteGrantsRequest{
        Identity: receiver,
        Scope: grant.Scope,
        Publisher: publisher,
      })
    }

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.grants")

    createStatus, createResponses, err := aap.CreateGrants(aapClient, url, createGrantsRequests)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }

    /*
    deleteStatus, deleteResponses, err := aap.DeleteGrants(aapClient, url, deleteGrantsRequests)

    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }
    */

    if createStatus == 200 /* && deleteStatus == 200 */ {
      var createGrants aap.CreateGrantsResponse
      _, restErr := bulky.Unmarshal(0, createResponses, &createGrants)
      if restErr != nil {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }
        c.AbortWithStatus(404)
        return
      }

      /*
      var deleteGrants aap.DeleteGrantsResponse
      _, restErr = bulky.Unmarshal(0, deleteResponses, &deleteGrants)
      if restErr != nil {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }
        c.AbortWithStatus(404)
        return
      }
      */

      c.Redirect(http.StatusFound, fmt.Sprintf("/access/grant?receiver=%s&publisher=%s", receiver, publisher))
      c.Abort()
      return
    }

    c.AbortWithStatus(404)
  }
  return gin.HandlerFunc(fn)
}
