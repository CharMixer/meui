package clients

import (
  "strings"
  "net/http"
  "reflect"
  "gopkg.in/go-playground/validator.v9"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/charmixer/idp/client"

  "github.com/charmixer/meui/app"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
  "github.com/charmixer/meui/utils"
  "github.com/charmixer/meui/validators"
)

type clientForm struct {
  Name        string `form:"clientname"  binding:"required" validate:"required,notblank"`
  Description string `form:"description" binding:"required" validate:"required,notblank"`
  IsPublic    []string `form:"is_public"`
}

const ClientFieldsKey = "client.fields"
const ClientErrorsKey = "client.errors"

const ClientNameKey = "clientname"
const ClientDescriptionKey = "description"
const ClientIsPublicKey = "is_public"

func ShowClient(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowClient",
    })

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    var clientName string
    var description string
    rf := session.Flashes(ClientFieldsKey)
    if len(rf) > 0 {
      fields := rf[0].(map[string][]string)
      for k, v := range fields {
        if k == ClientDescriptionKey && len(v) > 0 {
          description = strings.Join(v, ", ")
        }
        if k == ClientNameKey && len(v) > 0 {
          description = strings.Join(v, ", ")
        }
      }
    }

    errors := session.Flashes(ClientErrorsKey)
    err := session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      log.Debug(err.Error())
    }

    var errorClientName string
    var errorDescription string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {

        if k == ClientNameKey && len(v) > 0 {
          errorClientName = strings.Join(v, ", ")
        }

        if k == ClientDescriptionKey && len(v) > 0 {
          errorDescription = strings.Join(v, ", ")
        }

      }
    }

    c.HTML(http.StatusOK, "client.html", gin.H{
      "title": "Client",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      ClientNameKey: clientName,
      ClientDescriptionKey: description,
      "errorClientName": errorClientName,
      "errorDesecription": errorDescription,
      "clientUrl": config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.client"),
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitClient(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitClient",
    })

    var form clientForm
    err := c.Bind(&form)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    var isPublic bool = false
    if len(form.IsPublic) > 0 {
      isPublic = form.IsPublic[0] == "on"
    }

    identity := app.RequireIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    // Save values if submit fails
    fields := make(map[string][]string)
    fields[ClientNameKey] = append(fields[ClientNameKey], form.Name)
    fields[ClientDescriptionKey] = append(fields[ClientDescriptionKey], form.Description)

    session.AddFlash(fields, ClientFieldsKey)
    err = session.Save()
    if err != nil {
      log.Debug(err.Error())
    }

    errors := make(map[string][]string)
    validate := validator.New()
    validate.RegisterValidation("notblank", validators.NotBlank)
    err = validate.Struct(form)
    if err != nil {

      // Validation syntax is invalid
      if err,ok := err.(*validator.InvalidValidationError); ok{
        log.Debug(err.Error())
        c.AbortWithStatus(http.StatusInternalServerError)
        return
      }

      reflected := reflect.ValueOf(form) // Use reflector to reverse engineer struct
      for _, err := range err.(validator.ValidationErrors){

        // Attempt to find field by name and get json tag name
        field,_ := reflected.Type().FieldByName(err.StructField())
        var name string

        // If form tag doesn't exist, use lower case of name
        if name = field.Tag.Get("form"); name == ""{
          name = strings.ToLower(err.StructField())
        }

        switch err.Tag() {
        case "required":
            errors[name] = append(errors[name], "Field is required")
            break
        case "eqfield":
            errors[name] = append(errors[name], "Field should be equal to the "+err.Param())
            break
        case "notblank":
          errors[name] = append(errors[name], "Field is not allowed to be blank")
          break
        default:
            errors[name] = append(errors[name], "Field is invalid")
            break
        }
      }

    }

    if len(errors) > 0 {
      session.AddFlash(errors, ClientErrorsKey)
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

    idpClient := app.IdpClientUsingAuthorizationCode(env, c)

    status, _, err := idp.CreateClients(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection"), []idp.CreateClientsRequest{
      {
        Name: form.Name,
        Description: form.Description,
        IsPublic: isPublic,
      },
    })
    if err != nil {
      log.Debug("Client create failed")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      // Cleanup session
      session.Delete(ClientFieldsKey)
      session.Delete(ClientErrorsKey)
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      redirectTo := config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.clients.collection")
      log.WithFields(logrus.Fields{"redirect_to": redirectTo}).Debug("Redirecting")

      c.Redirect(http.StatusFound, redirectTo)
      c.Abort()
      return
    }

    // Deny by default. Failed to fill in the form correctly.
    submitUrl, err := utils.FetchSubmitUrlFromRequest(c.Request, nil)
    if err != nil {
      log.Debug(err.Error())
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }
    log.WithFields(logrus.Fields{"redirect_to": submitUrl}).Debug("Redirecting")
    c.Redirect(http.StatusFound, submitUrl)
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}