package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hudl/fargo"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// httpClient con timeout configurable para pings y peticiones al microservicio
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// getOutboundIP intenta averiguar la IP local "real" (no loopback)
func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func main() {
	// Instancia PocketBase
	app := pocketbase.New()

	// Variables de configuración (pueden venir de entorno)
	eurekaURL := os.Getenv("EUREKA_URL")
	if eurekaURL == "" {
		eurekaURL = "http://172.25.136.15:8761/eureka"
	}
	appName := os.Getenv("EUREKA_APP")
	if appName == "" {
		appName = "POCKETBASE-SERVER"
	}
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8090"
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	// Detectar IP y hostname
	ip, err := getOutboundIP()
	if err != nil {
		log.Printf("⚠️ No pude auto-detectar IP: %v, usaré loopback", err)
		ip = "127.0.0.1"
	}
	hostname, _ := os.Hostname()

	// Hook de bootstrap: registro inicial en Eureka
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		ec := fargo.NewConn(eurekaURL)
		client := &ec

		inst := &fargo.Instance{
			HostName:       hostname,
			Port:           port,
			App:            appName,
			IPAddr:         ip,
			VipAddress:     appName,
			Status:         fargo.UP,
			DataCenterInfo: fargo.DataCenterInfo{Name: fargo.MyOwn},
		}

		if err := client.RegisterInstance(inst); err != nil {
			log.Printf("⚠️ Eureka registro fallido (omitido): %v", err)
		} else {
			log.Println("✅ Registrado en Eureka")
		}

		return e.Next()
	})

	// Hook de serve: tus rutas
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Ruta de licencia proxy
		se.Router.POST("/api/custom/licencia", func(e *core.RequestEvent) error {
			// Leer body raw
			reqBody, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.BadRequestError("Error al leer el body de la solicitud", err)
			}

			// Reenviar al microservicio
			microResp, err := httpClient.Post(
				"http://localhost:8091/mostrar-licencia",
				"application/json",
				bytes.NewReader(reqBody),
			)
			if err != nil {
				return e.InternalServerError("Error al llamar al microservicio", err)
			}
			defer microResp.Body.Close()

			// Copiar cabeceras de respuesta
			for key, values := range microResp.Header {
				for _, v := range values {
					e.Response.Header().Add(key, v)
				}
			}

			// Stream del body tal cual
			return e.Stream(
				microResp.StatusCode,
				microResp.Header.Get("Content-Type"),
				microResp.Body,
			)
		})
		// Ruta de certificado proxy
		se.Router.POST("/api/custom/certificado", func(e *core.RequestEvent) error {
			// Leer body raw
			reqBody, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.BadRequestError("Error al leer el body de la solicitud", err)
			}

			// Reenviar al microservicio
			microResp, err := httpClient.Post(
				"http://localhost:8091/mostrar-certificado?filename=rellenable",
				"application/json",
				bytes.NewReader(reqBody),
			)
			if err != nil {
				return e.InternalServerError("Error al llamar al microservicio", err)
			}
			defer microResp.Body.Close()

			// Copiar cabeceras de respuesta
			for key, values := range microResp.Header {
				for _, v := range values {
					e.Response.Header().Add(key, v)
				}
			}

			// Stream del body tal cual
			return e.Stream(
				microResp.StatusCode,
				microResp.Header.Get("Content-Type"),
				microResp.Body,
			)
		})

		// Otras rutas (requieren autenticación)
		se.Router.POST("/api/myapp/settings", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, map[string]bool{"success": true})
		}).Bind(apis.RequireAuth())

		se.Router.POST("/api/custom/email", func(e *core.RequestEvent) error {
			type EmailData struct {
				Recipients []string `json:"recipients"`
				Subject    string   `json:"subject"`
				Body       string   `json:"body"`
			}
		
			var emailData EmailData
		
			// Leer el cuerpo de la solicitud
			reqBody, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.BadRequestError("Error al leer el body de la solicitud", err)
			}
		
			// Decodificar el JSON manualmente
			if err := json.NewDecoder(bytes.NewReader(reqBody)).Decode(&emailData); err != nil {
				return e.BadRequestError("Error al decodificar JSON", err)
			}
		
			// Obtener todos los emails de la colección 'usuarios'
			userRecords, err := e.App.FindAllRecords("usuarios");
			if err != nil {
				return e.InternalServerError("Error al obtener los correos válidos", err)
			}
		
			validEmails := map[string]bool{}
			for _, record := range userRecords {
				email := record.Get("email").(string)
				validEmails[email] = true
			}
		
			validRecipients := []string{}
			invalidRecipients := []string{}
		
			// Validar los correos de los destinatarios
			for _, email := range emailData.Recipients {
				if validEmails[email] {
					validRecipients = append(validRecipients, email)
				} else {
					invalidRecipients = append(invalidRecipients, email)
				}
			}
		
			if len(invalidRecipients) > 0 {
				log.Printf("❌ Emails no encontrados: %v", invalidRecipients)
			}
		
			if len(validRecipients) == 0 {
				return e.JSON(http.StatusBadRequest, map[string]any{
					"success": false,
					"message": "No se encontraron destinatarios válidos",
					"invalid": invalidRecipients,
				})
			}
		
			// Construir objeto para reenviar al microservicio
			forwardBody := map[string]any{
				"recipients": validRecipients,
				"subject":    emailData.Subject,
				"body":       emailData.Body,
			}
		
			// Codificar en JSON
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(forwardBody); err != nil {
				return e.InternalServerError("Error al codificar JSON", err)
			}
		
			// Enviar al microservicio
			microResp, err := httpClient.Post(
				"http://localhost:8017/api/v1/auth/send-email-invitation",
				"application/json",
				&buf,
			)
			if err != nil {
				return e.InternalServerError("Error al llamar al microservicio", err)
			}
			defer microResp.Body.Close()
		
			// Leer respuesta
			respBody, _ := io.ReadAll(microResp.Body)
		
			// Responder con la información
			return e.JSON(http.StatusOK, map[string]any{
				"success":           true,
				"invalidRecipients": invalidRecipients,
				"microserviceReply": string(respBody),
			})
		})
		
		

		se.Router.POST("/api/custom/emails", func(e *core.RequestEvent) error {
			type EmailData struct {
				Recipients []string `json:"recipients"`
				Subject    string   `json:"subject"`
				Body       string   `json:"body"`
			}
		
			var emailData EmailData
		
			// Leer el cuerpo de la solicitud
			reqBody, err := io.ReadAll(e.Request.Body)
			if err != nil {
				return e.BadRequestError("Error al leer el body de la solicitud", err)
			}
		
			// Decodificar el JSON manualmente
			if err := json.NewDecoder(bytes.NewReader(reqBody)).Decode(&emailData); err != nil {
				return e.BadRequestError("Error al decodificar JSON", err)
			}
		
			// Obtener todos los emails de la colección 'usuarios'
			userRecords, err := e.App.FindAllRecords("usuarios");
			if err != nil {
				return e.InternalServerError("Error al obtener los correos válidos", err)
			}
		
			validEmails := map[string]bool{}
			for _, record := range userRecords {
				email := record.Get("email").(string)
				validEmails[email] = true
			}
		
			validRecipients := []string{}
			invalidRecipients := []string{}
		
			// Validar los correos de los destinatarios
			for _, email := range emailData.Recipients {
				if validEmails[email] {
					validRecipients = append(validRecipients, email)
				} else {
					invalidRecipients = append(invalidRecipients, email)
				}
			}
		
			if len(invalidRecipients) > 0 {
				log.Printf("❌ Emails no encontrados: %v", invalidRecipients)
			}
		
			if len(validRecipients) == 0 {
				return e.JSON(http.StatusBadRequest, map[string]any{
					"success": false,
					"message": "No se encontraron destinatarios válidos",
					"invalid": invalidRecipients,
				})
			}
		
			// Construir objeto para reenviar al microservicio
			forwardBody := map[string]any{
				"recipients": validRecipients,
				"subject":    emailData.Subject,
				"body":       emailData.Body,
			}
		
			// Codificar en JSON
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(forwardBody); err != nil {
				return e.InternalServerError("Error al codificar JSON", err)
			}
		
			// Enviar al microservicio
			microResp, err := httpClient.Post(
				"http://localhost:8017/api/v1/auth/send-email-html",
				"application/json",
				&buf,
			)
			if err != nil {
				return e.InternalServerError("Error al llamar al microservicio", err)
			}
			defer microResp.Body.Close()
		
			// Leer respuesta
			respBody, _ := io.ReadAll(microResp.Body)
		
			// Responder con la información
			return e.JSON(http.StatusOK, map[string]any{
				"success":           true,
				"invalidRecipients": invalidRecipients,
				"microserviceReply": string(respBody),
			})
		})

		return se.Next()
	})

	// Hook de terminate: deregistro de Eureka
	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		log.Println("⚠️ PocketBase terminando, desregistrando de Eureka...")
		ec := fargo.NewConn(eurekaURL)
		client := &ec
		if err := client.DeregisterInstance(&fargo.Instance{HostName: hostname, App: appName}); err != nil {
			log.Printf("⚠️ Error desregistrando en Eureka: %v", err)
		} else {
			log.Println("🗑️ Desregistrado de Eureka exitosamente")
		}
		return te.Next()
	})

	// Cron job: heartbeat y ping a instancias cada 5 minutos
	app.Cron().MustAdd("eureka-health", "*/5 * * * *", func() {
		ec := fargo.NewConn(eurekaURL)
		client := &ec
		inst := &fargo.Instance{HostName: hostname, App: appName}

		// Heartbeat silencioso
		_ = client.HeartBeatInstance(inst)

		// Ping a todas las instancias registradas
		apps, err := client.GetApps()
		if err != nil {
			log.Printf("⚠️ Error leyendo apps de Eureka: %v", err)
			return
		}
		for _, a := range apps {
			for _, ins := range a.Instances {
				url := fmt.Sprintf("http://%s:%d", ins.IPAddr, ins.Port)
				resp, err := httpClient.Get(url)
				if err != nil {
					log.Printf("❌ %s/%s inaccesible (timeout %s): %v", a.Name, ins.HostName, httpClient.Timeout, err)
				} else {
					resp.Body.Close()
					log.Printf("✅ %s/%s reachable (status %d)", a.Name, ins.HostName, resp.StatusCode)
				}
			}
		}
	})

	// Otro cron de ejemplo
	app.Cron().MustAdd("hello", "*/2 * * * *", func() {
		log.Println("Hello!")
	})

	// Arranque de PocketBase
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
