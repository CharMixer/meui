package shadows

import (
  "net/url"
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  "golang.org/x/oauth2"
  oidc "github.com/coreos/go-oidc"

  bulky "github.com/charmixer/bulky/client"

  aap "github.com/charmixer/aap/client"
  //idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  "github.com/charmixer/meui/app"
)

type shadow struct {
  Id string
  Role string
  Nbf int64
  Exp int64
  DeleteUrl string
}

type uiData struct {
  Role string
  Shadows []shadow
}

func ShowShadows(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowShadows",
    })

    session := sessions.Default(c)

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    role, roleExists := c.GetQuery("role")

    if !roleExists {
      c.AbortWithStatus(http.StatusNotFound)
      log.Debug("Missing role in query")
      return
    }

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "shadows.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)
    //idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    var responses []bulky.Response
    var err error
    var status int
    var restErr []bulky.ErrorResponse
    var ui uiData

    ui.Role = role

    // fetch shadows

    callUrl := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.shadows.collection")
    status, responses, err = aap.ReadShadows(aapClient, callUrl, []aap.ReadShadowsRequest{
      {Shadow: role},
    })

    if err != nil {
      c.AbortWithStatus(404)
      log.Debug(err.Error())
      return
    }

    if status == http.StatusForbidden {
      c.AbortWithStatus(http.StatusForbidden)
      log.Debug("403 Forbidden access to /shadows")
      return
    }

    if status != http.StatusOK {
      c.AbortWithStatus(404)
      log.Debug("Unable to get status 200 from /shadows")
      return
    }

    var shadows aap.ReadShadowsResponse
    _, restErr = bulky.Unmarshal(0, responses, &shadows)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    deleteUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.shadows.delete"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    createUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.shadow"))
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    _createUrl := *createUrl
    q := _createUrl.Query()
    q.Add("role", ui.Role)
    _createUrl.RawQuery = q.Encode()

    for _,s := range shadows {
      _deleteUrl := *deleteUrl
      q := _deleteUrl.Query()
      q.Add("id", s.Identity)
      q.Add("role", s.Shadow)
      _deleteUrl.RawQuery = q.Encode()

      ui.Shadows = append(ui.Shadows, shadow{
        Id: s.Identity,
        Role: s.Shadow,
        Nbf: s.NotBefore,
        Exp: s.Expire,
        DeleteUrl: _deleteUrl.String(),
      })
    }

    c.HTML(200, "shadows.html", gin.H{
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "title": "Identities shadowing " + ui.Role,
      "created": ui,
      "createUrl": _createUrl.String(),
      "user": identity.Username,
      "name": identity.Name,
    })

  }
  return gin.HandlerFunc(fn)
}
