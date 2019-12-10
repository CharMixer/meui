package publishings

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  "golang.org/x/oauth2"
  oidc "github.com/coreos/go-oidc"

  bulky "github.com/charmixer/bulky/client"

  aap "github.com/opensentry/aap/client"
  _ "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"

  "github.com/opensentry/meui/app"
)

func ShowPublishings(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowPublishings",
    })

    session := sessions.Default(c)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

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
    // idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    var url string
    var responses []bulky.Response
    var err error
    var restErr []bulky.ErrorResponse

    // fetch publishes
    url = config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.publishes")
    _, responses, err = aap.ReadPublishes(aapClient, url, []aap.ReadPublishesRequest{
      {Publisher: receiver},
    })

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    var publishings aap.ReadPublishesResponse
    _, restErr = bulky.Unmarshal(0, responses, &publishings)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    c.HTML(200, "publishings.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "title": "Publishings",
      "receiver": receiver,
      "publishings": publishings,
    })

  }
  return gin.HandlerFunc(fn)
}
