package profiles

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
)

func ShowProfile(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowProfile",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    c.HTML(http.StatusOK, "profile.html", gin.H{
      "title": "Profile",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "id": identity.Id,
      "user": identity.Username,
      "password": identity.Password,
      "name": identity.Name,
      "email": identity.Email,
      "totp_required": identity.TotpRequired,
      "meUiUrl": config.GetString("meui.public.url"),
      "changeEmailUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.emailchange"),
      "changePasswordUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.password"),
      "profileDeleteUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.delete"),
      "setupTotpUrl": config.GetString("idpui.public.url") + config.GetString("idpui.public.endpoints.totp"),
      "logoutUrl": config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.logout"),
    })
    return
  }
  return gin.HandlerFunc(fn)
}
