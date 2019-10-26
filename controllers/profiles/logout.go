package profiles

import (
  "net/http"
  "net/url"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
)

func ShowLogout(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowLogout",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    urlLogout := config.GetString("hydra.public.url") + config.GetString("hydra.public.endpoints.logout")
    if urlLogout == "" {
      log.Debug("Missing config hydra.public.url + hydra.public.endpoints.logout")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    logoutUrl, err := url.Parse(urlLogout)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    idToken := app.IdTokenRaw(c)
    if idToken == "" {
      log.Debug("Missing raw id_token")
      c.AbortWithStatus(http.StatusUnauthorized)
      return
    }

    // Create session verifier
    state, err := app.CreateRandomStringWithNumberOfBytes(32);
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    q := logoutUrl.Query()
    q.Add("state", state)
    q.Add("id_token_hint", idToken)
    q.Add("post_logout_redirect_uri", "https://me.localhost/seeyoulater")
    logoutUrl.RawQuery = q.Encode()

    redirectTo := logoutUrl.String()
    log.WithFields(logrus.Fields{ "redirect_to":redirectTo }).Debug("Redirecting")
    c.Redirect(http.StatusFound, redirectTo)
    c.Abort()
    return
  }
  return gin.HandlerFunc(fn)
}