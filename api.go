package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"time"
)

type mailDto struct {
	Id          string `json:"id"`
	Date        string `json:"date"`
	Subject     string `json:"subject"`
	Data        string `json:"data"`
	To          string `json:"to"`
	IsRead      int    `json:"isRead"`
	From        string `json:"from"`
	Body        string `json:"body"`
	Cc          string `json:"cc"`
	Bcc         string `json:"bcc"`
	Rcpt        string `json:"rcpt"`
	MimeVersion string `json:"mimeVersion"`
	ContentType string `json:"contentType"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	clientOptions := options.Client().ApplyURI(os.Getenv("MONGO_URI"))
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB!")

	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.Static("/assets", "./assets")
	router.GET("/", func(c *gin.Context) {

		curl := "curl  --url 'smtp://" + os.Getenv("HOST") + ":" + os.Getenv("SMTP_PORT") + "' --user '" + os.Getenv("SMTP_USERNAME") + ":" + os.Getenv("SMTP_PASSWORD") + "' --mail-from " + os.Getenv("SMTP_USERNAME") + " --mail-rcpt " + os.Getenv("SMTP_USERNAME") + " --upload-file - <<EOF\n" +
			"From: My Inbox <" + os.Getenv("SMTP_USERNAME") + ">\n" +
			"To: Your Inbox <" + os.Getenv("SMTP_USERNAME") + ">\n" +
			"Subject: Test Mail\n" +
			"Content-Type: multipart/alternative; boundary=\"boundary-string\"\n" +
			"\n" +
			"--boundary-string\n" +
			"--boundary-string\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\n" +
			"Content-Transfer-Encoding: quoted-printable\n" +
			"Content-Disposition: inline\n" +
			"\n" +
			"Test Mail\n" +
			"\n" +
			"Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book\n" +
			"\n" +
			"--boundary-string\n" +
			"Content-Type: text/html; charset=\"utf-8\"\n" +
			"Content-Transfer-Encoding: quoted-printable\n" +
			"Content-Disposition: inline\n" +
			"\n" +
			"<!doctype html>\n" +
			"<html>\n" +
			"<head>\n" +
			"<meta http-equiv=\"Content-Type\" content=\"text/html; charset=UTF-8\">\n" +
			"</head>\n" +
			"<body style=\"font-family: sans-serif;\">\n" +
			"<div style=\"display: block; margin: auto; max-width: 600px;\" class=\"main\">\n" +
			"<h1 style=\"font-size: 18px; font-weight: bold; margin-top: 20px\">Test Mail</h1>\n" +
			"<p>Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book</p>\n" +
			"</div>\n" +
			"<style>\n" +
			".main { background-color: #EEE; }\n" +
			"a:hover { border-left-width: 1em; min-height: 2em; }\n" +
			"</style>\n" +
			"</body>\n" +
			"</html>\n" +
			"\n" +
			"--boundary-string--\n" +
			"EOF"

		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title":    "MailTracker",
			"curl":     curl,
			"host":     os.Getenv("HOST"),
			"port":     os.Getenv("SMTP_PORT"),
			"username": os.Getenv("SMTP_USERNAME"),
			"password": os.Getenv("SMTP_USERNAME"),
		})
	})
	router.GET("/api/mails", func(c *gin.Context) {
		var mails []mailDto
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		// order by date desc
		opts := options.Find()
		opts.SetSort(bson.D{{"_id", -1}})
		cur, err := collection.Find(context.TODO(), bson.D{}, opts)
		if err != nil {
			log.Fatal(err)
		}
		// has empty result empty array
		if cur.RemainingBatchLength() == 0 {
			c.JSON(http.StatusOK, gin.H{
				"data": mails,
			})
			return
		}

		for cur.Next(context.TODO()) {
			var mail mailDto

			err := cur.Decode(&mail)
			if err != nil {
				log.Fatal(err)
			}
			mail.Id = cur.Current.Lookup("_id").ObjectID().Hex()
			mails = append(mails, mail)
		}
		if err := cur.Err(); err != nil {
			log.Fatal(err)
		}
		err = cur.Close(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"data": mails,
		})
	})
	// get mail from iframe
	router.GET("/iframe/mails/:id", func(c *gin.Context) {
		objID, _ := primitive.ObjectIDFromHex(c.Param("id"))
		var mail mailDto
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		err := collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&mail)
		if err != nil {
			log.Fatal(err)
		}
		html := ""

		data := regexp.MustCompile(`(?s)<body.*?>(.*?)</body>`).FindStringSubmatch(mail.Body)
		if len(data) > 0 {
			html = data[1]
		}

		// when request query param return json
		if c.Query("json") == "true" {
			c.JSON(http.StatusOK, gin.H{
				"data": html,
			})
			return
		}

		// return template
		c.HTML(http.StatusOK, "iframe.tmpl", gin.H{
			"body": template.HTML(html),
		})
	})
	router.GET("/api/mails/:id", func(c *gin.Context) {
		objID, _ := primitive.ObjectIDFromHex(c.Param("id"))

		var mail mailDto
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		err := collection.FindOne(context.TODO(), bson.M{"_id": objID}).Decode(&mail)
		if err != nil {
			log.Fatal(err)
		}
		mail.Id = objID.Hex()
		_, err = collection.UpdateOne(context.TODO(), bson.M{"_id": objID}, bson.D{
			{"$set", bson.D{
				{"isRead", 1},
			},
			},
		})
		if err != nil {
			log.Fatal(err)
		}

		c.JSON(http.StatusOK, gin.H{
			"data": mail,
		})
	})
	router.DELETE("/api/mails/:id", func(c *gin.Context) {
		objID, _ := primitive.ObjectIDFromHex(c.Param("id"))
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		_, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Mail deleted",
		})
	})
	// delete all
	router.DELETE("/api/mails", func(c *gin.Context) {
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		_, err := collection.DeleteMany(context.TODO(), bson.M{})
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "All mails deleted",
		})
	})
	// read all
	router.PUT("/api/mails", func(c *gin.Context) {
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("mails")
		_, err := collection.UpdateMany(context.TODO(), bson.M{}, bson.D{
			{"$set", bson.D{
				{"isRead", 1},
			},
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "All mails read",
		})
	})
	// user and login routes

	type userDto = struct {
		Id       string `json:"id"`
		Salt     string `json:"salt"`
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	router.GET("/api/users", func(c *gin.Context) {
		var users []userDto
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("users")
		cur, err := collection.Find(context.TODO(), bson.D{})
		if err != nil {
			log.Fatal(err)
		}
		for cur.Next(context.TODO()) {
			var user userDto
			err := cur.Decode(&user)
			if err != nil {
				log.Fatal(err)
			}
			user.Id = cur.Current.Lookup("_id").ObjectID().Hex()
			users = append(users, user)
		}
		if err := cur.Err(); err != nil {
			log.Fatal(err)
		}
		err = cur.Close(context.TODO())
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"data": users,
		})
	})
	router.POST("/api/login", func(c *gin.Context) {
		var user userDto
		c.BindJSON(&user)

		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("users")
		plainPwd := user.Password
		// get username from users
		err := collection.FindOne(context.TODO(), bson.M{"username": user.Username}).Decode(&user)
		if err != nil {
			c.JSON(
				http.StatusUnauthorized,
				gin.H{
					"message": "Kullanıcı adı veya parola hatalı.",
				})
			return
		}

		byteHash := []byte(user.Password)
		err = bcrypt.CompareHashAndPassword(byteHash, []byte(plainPwd))
		if err != nil {
			c.JSON(
				http.StatusUnauthorized,
				gin.H{
					"message": "Kullanıcı adı veya parola hatalı.",
				})
			return
		}

		token := jwt.New(jwt.SigningMethodHS256)
		claims := make(jwt.MapClaims)
		claims["exp"] = time.Now().Add(time.Duration(360000)).Unix()
		claims["iat"] = time.Now().Unix()
		claims["sub"] = user.Salt

		tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
		if err != nil {
			log.Fatal(err)
		}

		c.JSON(
			http.StatusOK,
			gin.H{
				"data": gin.H{
					"token": tokenString,
					"user":  user,
				},
			},
		)
	})
	router.POST("/api/users", func(c *gin.Context) {
		var user userDto
		c.BindJSON(&user)

		rand.Seed(time.Now().UnixNano())
		b := make([]byte, 10+2)
		rand.Read(b)
		salt := fmt.Sprintf("%x", b)[2 : 10+2]
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("users")

		hashPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.MinCost)
		if err != nil {
			log.Println(err)
		}

		_, err = collection.InsertOne(context.TODO(), bson.D{
			{"username", user.Username},
			{"password", string(hashPassword)},
			{"salt", salt},
			{"role", user.Role},
		})
		if err != nil {
			log.Fatal(err)
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "User created",
		})
	})
	router.DELETE("/api/users/:id", func(c *gin.Context) {
		objID, _ := primitive.ObjectIDFromHex(c.Param("id"))
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("users")
		_, err := collection.DeleteOne(context.TODO(), bson.M{"_id": objID})
		if err != nil {
			log.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "User deleted",
		})
	})
	// update user
	router.PUT("/api/users/:id", func(c *gin.Context) {
		objID, _ := primitive.ObjectIDFromHex(c.Param("id"))
		var user userDto
		c.BindJSON(&user)
		collection := client.Database(os.Getenv("MONGO_TABLE_NAME")).Collection("users")

		_, err := collection.UpdateOne(context.TODO(), bson.M{"_id": objID}, bson.D{
			{"$set", bson.D{
				{"username", user.Username},
			},
			},
		})
		if err != nil {
			log.Fatal(err)
		}

		if len(user.Password) > 0 {
			h := md5.New()
			hashPassword, err := io.WriteString(h, user.Password)
			if err != nil {
				log.Fatal(err)
			}
			_, err = collection.UpdateOne(context.TODO(), bson.M{"_id": objID}, bson.D{
				{"$set", bson.D{
					{"password", hashPassword},
				},
				},
			})
			if err != nil {
				log.Fatal(err)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "User updated",
		})
	})

	router.Run()
}
