package invites

import (
  "net/url"
  "net/http"
  "time"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  //"github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"

  bulky "github.com/charmixer/bulky/client"
)

type InviteTemplate struct {
  IssuedAt string
  InvitedBy string
  Expires string
  Id string
  Email string
  GrantsUrl string
  SendUrl string
  SentAt string
}

func ShowInvites(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowInvites",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    status, responses, err := idp.ReadInvites(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.invites.collection"), nil)
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

    f := "2006-01-02 15:04:05" // Remder time format
    var uiCreatedInvites []InviteTemplate
    var uiSentInvites []InviteTemplate

    var invites idp.ReadInvitesResponse
    status, _ = bulky.Unmarshal(0, responses, &invites)
    if status == 200 {

      for _, invite := range invites {

        grantsUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.access.grant"))
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }
        q := grantsUrl.Query()
        q.Add("receiver", invite.Id)
        grantsUrl.RawQuery = q.Encode()

        sendUrl, err := url.Parse(config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.invites.send"))
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }
        q = sendUrl.Query()
        q.Add("id", invite.Id)
        sendUrl.RawQuery = q.Encode()

        sat := "n/a"
        if invite.SentAt > 0 {
          sat = time.Unix(invite.SentAt, 0).Format(f)
        }

        uiInvite := InviteTemplate{
          GrantsUrl: grantsUrl.String(),
          SendUrl:   sendUrl.String(),
          Id:        invite.Id,
          Email:     invite.Email,
          // InvitedBy: invite.InvitedBy,
          IssuedAt:  time.Unix(invite.IssuedAt, 0).Format(f),
          Expires:   time.Unix(invite.ExpiresAt, 0).Format(f),
          SentAt: sat,
        }
        uiCreatedInvites = append(uiCreatedInvites, uiInvite)

      }

    }

    c.HTML(http.StatusOK, "invites.html", gin.H{
      "title": "Invites",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      "created": uiCreatedInvites,
      "sent": uiSentInvites,
    })
  }
  return gin.HandlerFunc(fn)
}
