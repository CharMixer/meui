package resourceservers

import (
  "strings"
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
  "github.com/charmixer/meui/utils"

  bulky "github.com/charmixer/bulky/client"
)

type resourceServerDeleteForm struct {
  Id           string `form:"id" binding:"required" validate:"required,uuid"`
  RiskAccepted string `form:"risk_accepted"`
}

const ResourceServerDeleteErrorsKey = "resourceserver.delete.errors"
const ResourceServerAcceptRiskKey = "resourceserver.delete.risk_accepted"

func ShowResourceServerDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowResourceServerDelete",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    resourceServerToDeleteId := c.Query("id")
    if resourceServerToDeleteId == "" {
      log.Debug("Missing id")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    var resourceServer *idp.ResourceServer
    idpClient := app.IdpClientUsingAuthorizationCode(env, c)
    readRequest := []idp.ReadResourceServersRequest{ {Id: resourceServerToDeleteId} }
    status, responses, err := idp.ReadResourceServers(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection"), readRequest)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      var resp idp.ReadResourceServersResponse
      status, _ = bulky.Unmarshal(0, responses, &resp)
      if status == 200 {
        resourceServer = &resp[0]
      }

    }

    if resourceServer == nil {
      log.Debug("Resource Server does not exist")
      c.AbortWithStatus(http.StatusNotFound)
      return
    }

    session := sessions.Default(c)

    riskAccepted := session.Flashes(ResourceServerAcceptRiskKey)

    errors := session.Flashes(ResourceServerDeleteErrorsKey)
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

    c.HTML(http.StatusOK, "resourceserverdelete.html", gin.H{
      "title": "Delete Resource Server",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": resourceServer.Id,
      "username": identity.Username,
      "name": identity.Name,
      "RiskAccepted": riskAccepted,
      "errorRiskAccepted": errorRiskAccepted,
      "submitUrl": submitUrl,
      "resourceServer": resourceServer,
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitResourceServerDelete(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitResourceServerDelete",
    })

    var form resourceServerDeleteForm
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

        deleteRequest := []idp.DeleteResourceServersRequest{ {Id: form.Id} }
        status, responses, err := idp.DeleteResourceServers(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection"), deleteRequest)
        if err != nil {
          log.Debug(err.Error())
          c.AbortWithStatus(http.StatusInternalServerError)
          return
        }

        if status == 200 {

          var resp idp.DeleteResourceServersResponse
          status, _ := bulky.Unmarshal(0, responses, &resp)
          if status == 200 {

            // Cleanup session
            session.Delete(ResourceServerAcceptRiskKey)
            session.Delete(ResourceServerDeleteErrorsKey)
            err = session.Save()
            if err != nil {
              log.Debug(err.Error())
            }

            redirectTo := config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.resourceservers.collection")
            log.WithFields(logrus.Fields{"redirect_to": redirectTo}).Debug("Redirecting");
            c.Redirect(http.StatusFound, redirectTo)
            c.Abort()
            return
          }

        }

      }

    }

    // Deny by default
    session.AddFlash(form.RiskAccepted, ResourceServerAcceptRiskKey)
    session.AddFlash(errors, ResourceServerDeleteErrorsKey)
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