package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	b64 "encoding/base64"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hyson007/firebaseUpdate"
	"github.com/hyson007/gmailSenderOauth"
	"google.golang.org/api/gmail/v1"
)

var emailSubMap map[string]string
var phoneSubMap map[string]string
var gsvc *gmail.Service

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func isInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

//remove space, all small letter
func normaliseString(s string) string {
	temp := strings.ToLower(s)
	return strings.Replace(temp, " ", "", -1)
}

func init() {
	gsvc = gmailSenderOauth.NewOAuthGmailService()
}

func main() {

	//cache init
	emailSubMap = make(map[string]string)
	phoneSubMap = make(map[string]string)

	//Gin init
	r := gin.Default()
	r.LoadHTMLGlob("templates/*.gohtml")
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET"},
		AllowHeaders: []string{"Origin", "Content-Type", "ContentType",
			"Content-Length", "Accept-Encoding", "Authorization", "accept", "Cache-Control"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return origin == "*"
		},
		MaxAge: 12 * time.Hour,
	}))
	// var countries map[string]bool
	// var cities map[string]bool

	countries := make(map[string]bool)
	citiesInCountry := make(map[string][]string)
	citiesCoordinates := make(map[string]string)

	f, err := os.Open("worldcities.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		recordCountry := normaliseString(record[4])
		recordCity := normaliseString(record[1])
		recordLat := record[2]
		recordLng := record[3]
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		//insert country
		if !countries[recordCountry] {
			countries[recordCountry] = true
		}

		//insert citiesInCountry
		cityList := citiesInCountry[recordCountry]
		if !isInSlice(recordCity, cityList) {
			cityList = append(cityList, recordCity)
			citiesInCountry[recordCountry] = cityList
		}

		//insert citiesCoordinates
		value := recordLat + "," + recordLng
		_, ok := citiesCoordinates[recordCity]
		if !ok {
			citiesCoordinates[recordCity] = value
		}

	}

	r.GET("/cities/:country", func(c *gin.Context) {
		country := c.Param("country")
		cities, ok := citiesInCountry[country]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": "country not found"})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{"message": cities})
		}
	})

	r.GET("/countries", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": getKeys(countries),
		})
	})

	r.GET("/coordinates/:city", func(c *gin.Context) {
		city := c.Param("city")
		value, ok := citiesCoordinates[city]
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "city not found",
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": value,
			})
		}
	})

	r.GET("/subscription", func(c *gin.Context) {
		email, emailOk := c.GetQuery("email")
		phone, phoneOk := c.GetQuery("phone")
		docID, docIDOk := c.GetQuery("docID")
		if !docIDOk {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "docID not found",
			})
			return
		}
		if !emailOk && !phoneOk {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Internal Error, Email or Phone required",
			})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": "Subscription Successful",
			})

			if emailOk {

				emailSubMap[email] = uuid.New().String()
				data := docID + "__" + email + "__" + emailSubMap[email]
				encoded := b64.StdEncoding.EncodeToString([]byte(data))
				NotificationSubject := "Subject: Welcome to subscribe GoSchool earthquake notification\n"
				message := fmt.Sprintf(
					"Hello,\n\nYou have subscribed to GoSchool earthquake notification.\n\nThank you for using our service.\n\nGoSchool EarthQuake Notification Service\nKindly check the URL to confirm your subscription\nhttp://localhost:8080/verification/%s\n\nRegards,\nGoSchool EarthQuake Notification Service", encoded)
				log.Println("Email: "+email, "base64: "+encoded)
				// send email to user with link
				result, err := gmailSenderOauth.SendEmailOAUTH2(gsvc, email, NotificationSubject, message)
				if !result {
					log.Println("Error: " + err.Error())
				}

			}
			if phoneOk {
				phoneSubMap[phone] = uuid.New().String()[:4]
				data := docID + "__" + phone + "__" + phoneSubMap[phone]
				encoded := b64.StdEncoding.EncodeToString([]byte(data))
				log.Println("Phone: "+phone, "base64: "+encoded)
				// txt to user with link
			}
		}
		defer fmt.Println(emailSubMap, phoneSubMap)
	})

	r.GET("/verification/:token", func(c *gin.Context) {
		base64Token := c.Param("token")
		data, err := b64.StdEncoding.DecodeString(base64Token)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Internal Error, Invalid Token",
			})
			return
		}
		fmt.Println("data: " + string(data))
		decodeString := strings.Split(string(data), "__")
		if len(decodeString) != 3 {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Internal Error, Invalid Token",
			})
			log.Println("Invalid Token, len(decodeString) != 3")
			log.Println("Invalid Token: ", decodeString)
			return
		}
		docID := decodeString[0]
		emailOrPhone := decodeString[1]
		randomString := decodeString[2]
		errorString := "Invalid Token or expired Token"
		if strings.Contains(emailOrPhone, "@") {
			v, ok := emailSubMap[emailOrPhone]
			if !ok {
				// c.JSON(http.StatusBadRequest, gin.H{
				// 	"message": "Invalid Token or expired Token",
				// })
				c.HTML(http.StatusOK, "error.gohtml", gin.H{"errorString": errorString})
				log.Println("Invalid Token, emailSubMap doesn't contain email")
				log.Println("Invalid Token: ", decodeString)
				log.Println("emailSubMap: ", emailSubMap)
				return
			}
			if v == randomString {
				log.Println("valid email token")
				//update verified email in firestore status to true
				err := firebaseUpdate.UpdateRecord("subscriptions", docID, "email", true)
				if err != nil {
					log.Println("Error updating email in firestore")
					log.Fatal(err)
				}
				delete(emailSubMap, emailOrPhone)

			}
		} else {
			v, ok := phoneSubMap[emailOrPhone]
			if !ok {
				c.HTML(http.StatusOK, "error.gohtml", gin.H{"errorString": errorString})
				log.Println("Invalid Token, phoneSubMap doesn't contain phone")
				log.Println("Invalid Token: ", decodeString)
				log.Println("phoneSubMap: ", phoneSubMap)
				return
			}
			if v == randomString {
				log.Println("valid phone token")
				//update verified phone in db status to true
				delete(phoneSubMap, emailOrPhone)
				err := firebaseUpdate.UpdateRecord("subscriptions", docID, "phone", true)
				if err != nil {
					log.Println("Error updating email in firestore")
					log.Fatal(err)
				}
				c.HTML(http.StatusOK, "registration.gohtml", gin.H{"emailOrPhone": emailOrPhone})
			}
		}
		c.HTML(http.StatusOK, "registration.gohtml", gin.H{"emailOrPhone": emailOrPhone})
	})

	r.Run()
}
