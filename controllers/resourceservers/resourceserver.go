package resourceservers

import (
  "strings"
  "net/http"
  "reflect"
  "gopkg.in/go-playground/validator.v9"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gorilla/csrf"
  "github.com/gin-contrib/sessions"
  idp "github.com/opensentry/idp/client"

  "github.com/opensentry/meui/app"
  "github.com/opensentry/meui/config"
  "github.com/opensentry/meui/environment"
  "github.com/opensentry/meui/utils"
  "github.com/opensentry/meui/validators"
)

type resourceServerForm struct {
  Name        string `form:"resourceservername"  binding:"required" validate:"required,notblank"`
  Description string `form:"description" binding:"required" validate:"required,notblank"`
}

const ResourceServerFieldsKey = "resourceserver.fields"
const ResourceServerErrorsKey = "resourceserver.errors"

const ResourceServerNameKey = "resourceservername"
const ResourceServerDescriptionKey = "description"

func ShowResourceServer(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "ShowResourceServer",
    })

    identity := app.GetIdentity(c)
    if identity == nil {
      log.Debug("Missing Identity")
      c.AbortWithStatus(http.StatusForbidden)
      return
    }

    session := sessions.Default(c)

    var resourceServerName string
    var description string
    rf := session.Flashes(ResourceServerFieldsKey)
    if len(rf) > 0 {
      fields := rf[0].(map[string][]string)
      for k, v := range fields {
        if k == ResourceServerDescriptionKey && len(v) > 0 {
          description = strings.Join(v, ", ")
        }
        if k == ResourceServerNameKey && len(v) > 0 {
          resourceServerName = strings.Join(v, ", ")
        }
      }
    }

    errors := session.Flashes(ResourceServerErrorsKey)
    err := session.Save() // Remove flashes read, and save submit fields
    if err != nil {
      log.Debug(err.Error())
    }

    var errorResourceServerName string
    var errorDescription string

    if len(errors) > 0 {
      errorsMap := errors[0].(map[string][]string)
      for k, v := range errorsMap {

        if k == ResourceServerNameKey && len(v) > 0 {
          errorResourceServerName = strings.Join(v, ", ")
        }

        if k == ResourceServerDescriptionKey && len(v) > 0 {
          errorDescription = strings.Join(v, ", ")
        }

      }
    }

    c.HTML(http.StatusOK, "resourceserver.html", gin.H{
      "title": "Resource Server",
      "links": []map[string]string{
        {"href": "/public/css/dashboard.css"},
      },
      csrf.TemplateTag: csrf.TemplateField(c.Request),
      "provider": config.GetString("provider.name"),
      "id": identity.Id,
      "user": identity.Username,
      "name": identity.Name,
      ResourceServerNameKey: resourceServerName,
      ResourceServerDescriptionKey: description,
      "errorResourceServerName": errorResourceServerName,
      "errorDesecription": errorDescription,
      "resourceServerUrl": config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.resourceserver"),
    })
  }
  return gin.HandlerFunc(fn)
}

func SubmitResourceServer(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "SubmitResourceServer",
    })

    var form resourceServerForm
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

    // Save values if submit fails
    fields := make(map[string][]string)
    fields[ResourceServerNameKey] = append(fields[ResourceServerNameKey], form.Name)
    fields[ResourceServerDescriptionKey] = append(fields[ResourceServerDescriptionKey], form.Description)

    session.AddFlash(fields, ResourceServerFieldsKey)
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
      session.AddFlash(errors, ResourceServerErrorsKey)
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

    status, _, err := idp.CreateResourceServers(idpClient, config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection"), []idp.CreateResourceServersRequest{
      {
        Name: form.Name,
        Description: form.Description,
        Audience: form.Name, // FIXME: This needs to be user input and properly handled for failure
      },
    })
    if err != nil {
      log.Debug("Resource server create failed")
      c.AbortWithStatus(http.StatusInternalServerError)
      return
    }

    if status == 200 {

      // Cleanup session
      session.Delete(ResourceServerFieldsKey)
      session.Delete(ResourceServerErrorsKey)
      err = session.Save()
      if err != nil {
        log.Debug(err.Error())
      }

      redirectTo := config.GetString("meui.public.url") + config.GetString("meui.public.endpoints.resourceservers.collection")
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