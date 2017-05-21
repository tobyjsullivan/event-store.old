package main

import (
	"os"

	"github.com/codegangsta/negroni"
	"net/http"
	"github.com/gorilla/mux"
	"fmt"
    "encoding/json"
    "sync"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "github.com/aws/aws-sdk-go/aws"
    "strconv"
    "errors"
    "log"
    "github.com/aws/aws-sdk-go/aws/credentials"
)

var (
    locks map[string]sync.Mutex
    svc *dynamodb.DynamoDB
    logger *log.Logger
    dynamoTable string
)

func init() {
    logger = log.New(os.Stdout, "[event-store] ", 0)
    logger.Println("Logger initialized")

    require("AWS_ACCESS_KEY_ID")
    require("AWS_SECRET_ACCESS_KEY")
    sess := session.Must(session.NewSession(&aws.Config{
        Credentials: credentials.NewEnvCredentials(),
        Region: aws.String(require("AWS_REGION")),
    }))
    svc = dynamodb.New(sess)
    logger.Println("DynamoDB connected")

    dynamoTable = os.Getenv("DYNAMODB_TABLE")

    locks = make(map[string]sync.Mutex)
    logger.Println("Locks initialized")
}

func require(envvar string) string {
    v := os.Getenv(envvar)
    if v == "" {
        panic(fmt.Sprintf("Must set %s", envvar))
    }

    return v
}

func main() {
	r := buildRoutes()

	n := negroni.New()
	n.UseHandler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	n.Run(":" + port)
}

func buildRoutes() http.Handler {
	r := mux.NewRouter()
    r.HandleFunc("/", helloWorldHandler).Methods("GET")
	r.HandleFunc("/{streamId}", writeNextEventHandler).Methods("POST")
	r.HandleFunc("/{streamId}/{version}", writeEventWithVersionHandler).Methods("POST")

	return r
}

func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "The service is online\n")
}

func writeNextEventHandler(w http.ResponseWriter, r *http.Request) {
    logger.Println("Received request: Commit Event")
    vars := mux.Vars(r)

    entityId := vars["streamId"]
    if entityId == "" {
        logger.Println("Invalid Stream ID")
        http.Error(w, "Invalid Stream ID", http.StatusBadRequest)
        return
    }
    logger.Println("Stream ID:", entityId)

    decoder := json.NewDecoder(r.Body)
    var req addEventRequest
    if err := decoder.Decode(&req); err != nil {
        logger.Println("Failed to parse request.", err.Error())
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    logger.Println("Storing event.")
    if err := storeNextEvent(entityId, &req); err != nil {
        logger.Println("Error storing event.", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    logger.Println("Successfully stored event")
    w.Write([]byte("Success"))
}

func writeEventWithVersionHandler(w http.ResponseWriter, r *http.Request) {
    logger.Println("Received request: Commit Event")
    vars := mux.Vars(r)

    entityId := vars["streamId"]
    if entityId == "" {
        logger.Println("Invalid Stream ID")
        http.Error(w, "Invalid Stream ID", http.StatusBadRequest)
        return
    }
    logger.Println("Stream ID:", entityId)

    version, err := strconv.Atoi(vars["version"])
    if err != nil {
        logger.Println("Invalid version.", err.Error())
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    logger.Println("Version:", version)

    decoder := json.NewDecoder(r.Body)
    var req addEventRequest
    if err := decoder.Decode(&req); err != nil {
        logger.Println("Failed to parse request.", err.Error())
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    logger.Println("Storing event.")
    if err := storeEventWithVersion(entityId, version, &req); err != nil {
        logger.Println("Error storing event.", err.Error())
        if _, ok := err.(*versionConflictError); ok {
            http.Error(w, err.Error(), http.StatusConflict)
            return
        }

        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    logger.Println("Successfully stored event")
    w.Write([]byte("Success"))
}

type addEventRequest struct {
    Type string `json:"type"`
    Data string `json:"data"`
}

func storeNextEvent(entityId string, req *addEventRequest) error {
    mx := locks[entityId]
    mx.Lock()
    defer mx.Unlock()
    logger.Println("Lock obtained.")

    lastVer, err := getLastVersion(entityId)
    if err != nil {
        logger.Println("Error getting last version.", err.Error())
        return err
    }

    version := lastVer + 1

    logger.Println("Putting Item to DynamoDB")
    _, err = svc.PutItem(&dynamodb.PutItemInput{
        TableName: aws.String(dynamoTable),
        Item: map[string]*dynamodb.AttributeValue{
            "Entity ID": { S: aws.String(entityId) },
            "Version": { N: aws.String(strconv.Itoa(version)) },
            "Event Type": { S: aws.String(req.Type) },
            "Data": { S: aws.String(req.Data) },
        },
    })
    if err != nil {
        logger.Println("PutItem failed.", err.Error())
        return err
    }

    logger.Println("PutItem succeeded.")

    return nil
}

func storeEventWithVersion(entityId string, version int, req *addEventRequest) error {
    mx := locks[entityId]
    mx.Lock()
    defer mx.Unlock()
    logger.Println("Lock obtained.")

    lastVer, err := getLastVersion(entityId)
    if err != nil {
        logger.Println("Error getting last version.", err.Error())
        return err
    }

    if version != lastVer + 1 {
        logger.Println("The specified version is incorrect")
        return &versionConflictError{currentVersion: lastVer, msg: fmt.Sprintf("The requested version is invalid. Current Version: %d", lastVer)}
    }

    logger.Println("Putting Item to DynamoDB")
    _, err = svc.PutItem(&dynamodb.PutItemInput{
        TableName: aws.String(dynamoTable),
        Item: map[string]*dynamodb.AttributeValue{
            "Entity ID": { S: aws.String(entityId) },
            "Version": { N: aws.String(strconv.Itoa(version)) },
            "Event Type": { S: aws.String(req.Type) },
            "Data": { S: aws.String(req.Data) },
        },
    })
    if err != nil {
        logger.Println("PutItem failed.", err.Error())
        return err
    }

    logger.Println("PutItem succeeded.")

    return nil
}

func getLastVersion(entityId string) (int, error) {
    logger.Println("Querying DynamoDB.")
    res, err := svc.Query(&dynamodb.QueryInput{
        TableName: aws.String(dynamoTable),
        ConsistentRead: aws.Bool(true),
        ExpressionAttributeNames: map[string]*string{ "#entityColumn": aws.String("Entity ID")},
        ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{ ":entityId": { S: aws.String(entityId) } },
        ProjectionExpression: aws.String("Version"),
        KeyConditionExpression: aws.String("#entityColumn = :entityId"),
        Limit: aws.Int64(1),
        ScanIndexForward: aws.Bool(false),
    })
    if err != nil {
        logger.Println("DynamoDB query failed.", err.Error())
        return 0, err
    }

    logger.Println("DynamoDB query succeeded.")

    version := 0
    for _, item := range res.Items {
        v := item["Version"]
        if v == nil || v.N == nil || aws.StringValue(v.N) == "" {
            logger.Println("Couldn't find version in received event", item)
            return 0, errors.New("Received an item without a version.")
        }

        parsed, err := strconv.Atoi(aws.StringValue(v.N))
        if err != nil {
            logger.Println("Error parsing version.", err.Error())
            return 0, err
        }

        if parsed > version {
            version = parsed
        }
    }
    logger.Println("Determined last version.", version)

    return version, nil
}

type versionConflictError struct {
    currentVersion int
    msg string
}

func (e *versionConflictError) Error() string {
    return e.msg
}
