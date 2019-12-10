package clients

import (
  "strings"
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
  "github.com/opensentry/meui/utils"

  bulky "github.com/charmixer/bulky/client"
)

type clientDeleteForm struct {
  Id           string `form:"id" binding:"required" validate:"required,uuid"`
  RiskAccepted string `form:"risk_accepted"`
}

const ClientDeleteErrorsKey = "client.delete.errors"
const ClientAcceptRiskKey = "client.delete.risk_accepted"

func ShowClientDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowClientDelete",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    clientToDeleteId := c.Query("id")
    if clientToDeleteId == "" {
      log.Debug("Missing id")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    var client *idp.Client
    idpClient := app.IdpClientUsingAuthorizationCode(env, c)
    readRequest := []idp.ReadClientsRequest{ {Id: clientToDeleteId} }
    status, responses, err := idp.ReadClients(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection"), readRequest)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      var resp idp.ReadClientsResponse
      status, _ = bulky.Unmarshal(0, responses, &resp)
      if status == 200 {
        client = &resp[0]
      }

    }

    if client == nil {
      log.Debug("Client does not exist")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    session := sessions.Default(c)

    riskAccepted := session.Flashes(ClientAcceptRiskKey)

    errors := session.Flashes(ClientDeleteErrorsKey)
    err = session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      log.Debug(err.Error())
    }

    var errorRiskAccepted string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {

        if k == "errorRiskAccepted" && len(v) > 0 {
          errorRiskAccepted = strings.Join(v, ", ")
        }

      }
    }

    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    c.HTML(http.StatusOK, "clientdelete.html", gin.H{
      "title": "Delete Client",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": client.Id,
      "username": identity.Username,
      "name": identity.Name,
      "RiskAccepted": riskAccepted,
      "errorRiskAccepted": errorRiskAccepted,
      "submitUrl": submitUrl,
      "client": client,
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitClientDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitClientDelete",
    })

    var form clientDeleteForm
    err := c.Bind(&form)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)
    errors := make(map[string][]string)

    riskAccepted := len(form.RiskAccepted) > 0

    if riskAccepted == false {
      errors["errorRiskAccepted"] = append(errors["errorRiskAccepted"], "You have not accepted the risk")
    }

    if len(errors) <= 0  {

      if riskAccepted == true {

        idpClient := app.IdpClientUsingAuthorizationCode(env, c)

        deleteRequest := []idp.DeleteClientsRequest{ {Id: form.Id} }
        status, responses, err := idp.DeleteClients(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection"), deleteRequest)
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }

        if status == 200 {

          var resp idp.DeleteClientsResponse
          status, _ := bulky.Unmarshal(0, responses, &resp)
          if status == 200 {

            // Cleanup session
            session.Delete(ClientAcceptRiskKey)
            session.Delete(ClientDeleteErrorsKey)
            err = session.Save()
            if err != nil {
              log.Debug(err.Error())
            }

            redirectTo := config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.clients.collection")
            log.WithFields(logrus.Fields{"redirect_to": redirectTo}).Debug("Redirecting");
            c.Redirect(http.StatusFound, redirectTo)
            c.Abort()
            return
          }

        }

      }

    }

    // Deny by default
    session.AddFlash(form.RiskAccepted, ClientAcceptRiskKey)
    session.AddFlash(errors, ClientDeleteErrorsKey)
    err = session.Save()
    if err != nil {
      log.Debug(err.Error())
    }

    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
    c.Redirect(http.StatusFound, submitUrl)
    c.Abort()
    return

  }
  return gin.HandlerFunc(fn)
}