package ajax

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "github.com/gin-contrib/sessions"
  "golang.org/x/oauth2"
  oidc "github.com/coreos/go-oidc"
  bulky "github.com/charmixer/bulky/client"
  idp "github.com/charmixer/idp/client"
  "github.com/charmixer/meui/config"
  "github.com/charmixer/meui/environment"
  "fmt"
)

func GetIdentities(env *environment.State) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)

    session := sessions.Default(c)

    query, queryExists := c.GetQuery("q")

    if !queryExists {
      c.AbortWithStatus(404)
      log.Debug("Missing query")
      return
    }

    // NOTE: Maybe session is not a good way to do this.
    // 1. The user access /me with a browser and the access token / id token is stored in a session as we cannot make the browser redirect with Authentication: Bearer <token>
    // 2. The user is using something that supplies the access token and id token directly in the headers. (aka. no need for the session)
    var idToken *oidc.IDToken
    idToken = session.Get(environment.SessionIdTokenKey).(*oidc.IDToken)
    if idToken == nil {
      c.AbortWithStatus(http.StatusNotFound)
      log.Debug("Missing idToken")
      return
    }

    var accessToken *oauth2.Token
    accessToken = session.Get(environment.SessionTokenKey).(*oauth2.Token)
    idpClient := idp.NewIdpClientWithUserAccessToken(env.HydraConfig, accessToken)

    var url string
    var responses []bulky.Response
    var err error
    var status int
    var restErr []bulky.ErrorResponse

    // fetch identities

    url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.identities.collection")
    status, responses, err = idp.ReadIdentities(idpClient, url, []idp.ReadIdentitiesRequest{ {Search: query} })

    if !checkRestResponse(url, status, err, c, log) {
      return
    }

    var identities idp.ReadIdentitiesResponse
    _, restErr = bulky.Unmarshal(0, responses, &identities)
    if len(restErr) > 0 {
      for _,e := range restErr {
        // TODO show user somehow
        log.Debug("Rest error: " + e.Error)
      }

      c.AbortWithStatus(404)
      return
    }

    var readHumansRequests []idp.ReadHumansRequest
    var readClientsRequests []idp.ReadClientsRequest
    var readResourceServersRequests []idp.ReadResourceServersRequest
    var readRolesRequests []idp.ReadRolesRequest
    var readInvitesRequests []idp.ReadInvitesRequest

    for _,i := range identities {
      for _,l := range i.Labels {
        switch (l) {
          case "Human":
            readHumansRequests = append(readHumansRequests, idp.ReadHumansRequest{
              Id: i.Id,
            })
          break
          case "ResourceServer":
            readResourceServersRequests = append(readResourceServersRequests, idp.ReadResourceServersRequest{
              Id: i.Id,
            })
          break
          case "Client":
            readClientsRequests = append(readClientsRequests, idp.ReadClientsRequest{
              Id: i.Id,
            })
          break
          case "Role":
            readRolesRequests = append(readRolesRequests, idp.ReadRolesRequest{
              Id: i.Id,
            })
          break
          case "Invite":
            readInvitesRequests = append(readInvitesRequests, idp.ReadInvitesRequest{
              Id: i.Id,
            })
          break
        }
      }
    }

    var idToNameMap = make(map[string]string)

    // fetch humans

    if len(readHumansRequests) > 0 {
      url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.humans.collection")
      status, responses, err = idp.ReadHumans(idpClient, url, readHumansRequests)

      if !checkRestResponse(url, status, err, c, log) {
        return
      }

      for i,_ := range responses {
        var humans idp.ReadHumansResponse
        _, restErr = bulky.Unmarshal(i, responses, &humans)
        if len(restErr) > 0 {
          for _,e := range restErr {
            // TODO show user somehow
            log.Debug("Rest error: " + e.Error)
          }

          c.AbortWithStatus(404)
          return
        }

        for _,h := range humans {
          idToNameMap[h.Id] = h.Name
        }
      }
    }


    // fetch clients

    if len(readClientsRequests) > 0 {
      url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.clients.collection")
      status, responses, err = idp.ReadClients(idpClient, url, readClientsRequests)

      if !checkRestResponse(url, status, err, c, log) {
        return
      }


      for i,_ := range responses {
        var clients idp.ReadClientsResponse
        _, restErr = bulky.Unmarshal(i, responses, &clients)
        if len(restErr) > 0 {
          for _,e := range restErr {
            // TODO show user somehow
            log.Debug("Rest error: " + e.Error)
          }

          c.AbortWithStatus(404)
          return
        }

        for _,c := range clients {
          idToNameMap[c.Id] = c.Name
        }
      }
    }


    // fetch resource servers

    if len(readResourceServersRequests) > 0 {
      url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.resourceservers.collection")
      status, responses, err = idp.ReadResourceServers(idpClient, url, readResourceServersRequests)

      if !checkRestResponse(url, status, err, c, log) {
        return
      }

      for i,_ := range responses {
        var resourceServer idp.ReadClientsResponse
        _, restErr = bulky.Unmarshal(i, responses, &resourceServer)
        if len(restErr) > 0 {
          for _,e := range restErr {
            // TODO show user somehow
            log.Debug("Rest error: " + e.Error)
          }

          c.AbortWithStatus(404)
          return
        }

        for _,r := range resourceServer {
          idToNameMap[r.Id] = r.Name
        }
      }
    }


    // fetch roles

    if len(readRolesRequests) > 0 {
      url = config.GetString("idp.public.url") + config.GetString("idp.public.endpoints.roles.collection")
      status, responses, err = idp.ReadRoles(idpClient, url, readRolesRequests)

      if !checkRestResponse(url, status, err, c, log) {
        return
      }

      for i,_ := range responses {
        var roles idp.ReadRolesResponse
        _, restErr = bulky.Unmarshal(i, responses, &roles)
        if len(restErr) > 0 {
          for _,e := range restErr {
            // TODO show user somehow
            log.Debug("Rest error: " + e.Error)
          }

          c.AbortWithStatus(404)
          return
        }

        for _,r := range roles {
          idToNameMap[r.Id] = r.Name
        }
      }
    }




    type result struct {
      Success bool `json:"success"`
      Results []map[string]string `json:"results"`
    }

    var r result
    r.Success = true
    for _,i := range identities {
      name := "N/A"
      if v, found := idToNameMap[i.Id]; found {
        name = v
      }

      r.Results = append(r.Results, map[string]string{
        "name": fmt.Sprintf("%s (%s)", name, i.Id),
        "value": i.Id,
        "text": fmt.Sprintf("%s (%s)", name, i.Id),
      })
    }

    c.JSON(200, r)
  }
  return gin.HandlerFunc(fn)
}

func checkRestResponse(url string, status int, err error, c *gin.Context, log *logrus.Entry) (bool){
  if err != nil {
    c.AbortWithStatus(404)
    log.Debug(err.Error())
    return false
  }

  if status == http.StatusForbidden {
    c.AbortWithStatus(403)
    log.Debug("Got status forbidden from "+url)
    return false
  }

  if status != http.StatusOK {
    c.AbortWithStatus(404)
    log.Debug("Unable to get status 200 from "+url)
    return false
  }

  return true
}
