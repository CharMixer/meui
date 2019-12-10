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
  f "github.com/go-playground/form"
  "fmt"
)

type publishForm struct {
  Scope         string
  Title         string
  Description   string
}

func ShowPublish(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowPublish",
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
      log.Debug("Missing receiver")
      c.AbortWithStatus(404)
      return
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

    //var accessToken *oauth2.Token
    //accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    //aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)
    //idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    c.HTML(200, "publish.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "provider": config.GetString("provider.name"),
      "title": "Publish scope",
      "receiver": receiver,
    })

  }
  return gin.HandlerFunc(fn)
}

func SubmitPublish(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitPublish",
    })

    session := sessions.Default(c)

    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "publish.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    receiver, receiverExists := c.GetQuery("receiver")

    if !receiverExists  {
      log.WithFields(logrus.Fields{
        "receiver": receiver,
      }).Debug("receiver must exists")
      c.AbortWithStatus(404)
      return
    }

    var form publishForm
    c.Request.ParseForm()

    decoder := f.NewDecoder()
    err := decoder.Decode(&form, c.Request.Form)
    if err != nil {
      log.Panic(err)
      c.AbortWithStatus(404)
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)

    createPublishesRequest := aap.CreatePublishesRequest{
      Publisher: receiver,
      Scope: form.Scope,
      Title: form.Title,
      Description: form.Description,
    }

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.publishes")

    createStatus, createResponses, err := aap.CreatePublishes(aapClient, url, []aap.CreatePublishesRequest{createPublishesRequest})
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(404)
      return
    }

    if createStatus == 200 {
      var createPublishes aap.CreatePublishesResponse
      _, restErr := bulky.Unmarshal(0, createResponses, &createPublishes)
      if restErr != nil {
        for _,e := range restErr {
          // TODO show user somehow
          log.Debug("Rest error: " + e.Error)
        }
        c.AbortWithStatus(404)
        return
      }

      c.Redirect(http.StatusFound, fmt.Sprintf("/publishings?receiver=%s", receiver))
      c.Abort()
      return
    }

    c.AbortWithStatus(404)
  }
  return gin.HandlerFunc(fn)
}
