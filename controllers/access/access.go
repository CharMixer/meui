package access

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

  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
)

type newAccessForm struct {
  Scope       string `form:"scope" binding:"required"`
}

func ShowAccess(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowAccess",
    })

    session := sessions.Default(c)

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "access_new.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.scopes")
    _, responses, _ := aap.ReadScopes(aapClient, url, nil)

    var ok aap.ReadScopesResponse
    _, restErr := bulky.Unmarshal(0, responses, &ok)
    if restErr != nil {
      for _,e := range restErr {
        // TODO show user somehow
        log.Println("Rest error: " + e.Error)
      }
    }

    c.HTML(200, "access.html", gin.H{
      "title": "Access",
      "scopes": ok,
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "idpUiUrl": config.GetString("meui.public.url"),
      "aapUiUrl": config.GetString("aapui.public.url"),
    })
  }
  return gin.HandlerFunc(fn)
}


func ShowAccessNew(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowAccessNew",
    })

    c.HTML(200, "access_new.html", gin.H{
      "title": "Create new scope",
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "idpUiUrl": config.GetString("meui.public.url"),
      "aapUiUrl": config.GetString("aapui.public.url"),
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitAccessNew(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowAccess",
    })

    var form newAccessForm
    err := c.Bind(&form)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    session := sessions.Default(c)

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.HTML(http.StatusNotFound, "access_new.html", gin.H{"error": "Identity not found"})
      c.Abort()
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    aapClient := aap.NewAapClientWithUserAccessToken(env.HydraConfig, accessToken)

    var createScopesRequests []aap.CreateScopesRequest
    createScopesRequests = append(createScopesRequests, aap.CreateScopesRequest{
      Scope:               form.Scope,
    })

    url := config.GetString("aap.public.url") + config.GetString("aap.public.endpoints.scopes")

    _, responses, err := aap.CreateScopes(aapClient, url, createScopesRequests)

    var ok aap.CreateScopesResponse
    _, restErr := bulky.Unmarshal(0, responses, &ok)

    if restErr != nil {
      c.HTML(http.StatusOK, "access_new.html", gin.H{
        "title": "Create new scope",
        "errors": restErr,
        "links": []map[string]string{
          {"href": "/public/css/dashboard.css"},
        },
        "idpUiUrl": config.GetString("meui.public.url"),
        "aapUiUrl": config.GetString("aapui.public.url"),
      })
      c.Abort()
      return
    }

    c.Redirect(http.StatusFound, "/access")
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}
