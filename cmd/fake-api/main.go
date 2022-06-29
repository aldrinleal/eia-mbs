package main

import (
	"fmt"
	"github.com/Jeffail/gabs/v2"
	"github.com/aldrinleal/eia-mbs/util"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
	"github.com/shurcooL/go-goon"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"io"
	"net/http"
)

var (
	ginLambda *ginadapter.GinLambda
	engine    *gin.Engine
)

func init() {
	log.Infof("Initializing")

	engine = gin.Default()

	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion(util.EnvIf("AWS_DEFAULT_REGION", "AWS_REGION", "us-west-2"))))

	rekognitionClient := rekognition.New(sess)

	engine.POST("/api/v1/images", func(c *gin.Context) {
		imageFile, _ := c.FormFile("file")

		body := gabs.New()

		body.Set(0, "code")
		body.Set("", "message")
		body.Set("ObjectDetectionPrediction", "data", "type")
		body.Set(0.0007536411285400391, "data", "latency", "preprocess_s")
		body.Set(0.37453532218933105, "data", "latency", "infer_s")
		body.Set(0.000008344650268554688, "data", "latency", "postprocess_s")
		body.Set(0.0003764629364013672, "data", "latency", "serialize_s")

		file, err := imageFile.Open()

		if nil != err {
			c.Error(err)

			return
		}

		defer file.Close()

		imageBytes, err := io.ReadAll(file)

		if nil != err {
			c.Error(err)

			return
		}

		detectLabelsOutput, err := rekognitionClient.DetectLabels(&rekognition.DetectLabelsInput{
			Image:         &rekognition.Image{Bytes: imageBytes},
			MinConfidence: aws.Float64(0.3),
		})

		if nil != err {
			c.Error(err)

			return
		}

		for labelIndex, label := range detectLabelsOutput.Labels {
			for i, instance := range label.Instances {
				predictionKey := fmt.Sprintf("%s-%d", label.Name, i)

				body.Set(*instance.Confidence, "data", "predictions", predictionKey, "score")
				body.Set(*label.Name, "data", "predictions", predictionKey, "labelName")
				body.Set(labelIndex, "data", "predictions", predictionKey, "labelIndex")
				body.Set(*instance.BoundingBox.Top, "data", "predictions", predictionKey, "coordinates", "xmin")
				body.Set(*instance.BoundingBox.Top+*instance.BoundingBox.Height, "data", "predictions", predictionKey, "coordinates", "xmax")

			}
		}

		/*
			{
			  "code": 0,
			  "message": "",
			  "data": {
			    "predictions": {
			      "3ae97040-18a0-4431-a63e-98adfbe3b179": {
			        "score": 0.9007511138916016,
			        "labelName": "Flat Coated Retriever",
			        "labelIndex": 4,
			        "defectId": 2398650,
			        "coordinates": {
			          "xmin": 111,
			          "ymin": 31,
			          "xmax": 643,
			          "ymax": 545
			        }
			      }
			    },
			    "type": "ObjectDetectionPrediction",
			    "latency": {
			      "preprocess_s": 0.0007536411285400391,
			      "infer_s": 0.37453532218933105,
			      "postprocess_s": 0.000008344650268554688,
			      "serialize_s": 0.0003764629364013672
			    }
			  }
			}
		*/

		c.JSON(200, body.Data())
	})

	ginLambda = ginadapter.New(engine)
}

func main() {
	if util.IsRunningOnLambda() {
		lambda.Start(func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
			log.Infof("req: %s", goon.Sdump(req))

			return ginLambda.ProxyWithContext(ctx, req)
		})
	} else {
		log.Fatalf("Oops", http.ListenAndServe(":"+util.EnvIf("PORT", "8000"), engine))
	}
}
