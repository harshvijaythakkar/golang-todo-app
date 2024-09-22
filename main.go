package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"

	// mgo "gopkg.in/mgo.v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var rnd *renderer.Render
var collection *mongo.Collection
// var client *mongo.Client
var ctx = context.TODO()

const (
	hostName		string = "mongodb://localhost:27017"
	dbName			string = "demo_todo"
	collectionName	string = "todo"
	port			string = ":9000"
)

// brew services start mongodb/brew/mongodb-community
// mongosh

/*
show collections
show tables
show databases

use demo_todo

db.todo.insertOne( { title: "buy milk",  Completed: false, CreatedAt: new Date()} ) // insert new record

db.todo.find() // find all documents

db.todo.find({'completed':false}) // find docuemt which has completed=false

db.todo.deleteMany({}) // delete all documents

db.todo.deleteMany( { title: "buy juice" } ) // delete all documents with title="buy milk"

db.todo.deleteOne( { title: "buy milk" } ) // delete first document with title="buy milk"

*/

type (
	// mongodb
	todoModel struct {
		Id			primitive.ObjectID `bson:"_id,omitempty"`
		Title		string `bson:"title"`
		Completed 	bool `bson:"completed"`
		CreatedAt	time.Time `bson:"createdAt"`
	}

	// UI render
	todo struct {
		Id			string `json:"id"`
		Title		string `json:"title"`
		Completed	bool `json:"completed"`
		CreatedAt	time.Time `json:"created_at"`
	}
)


func init() {
	rnd = renderer.New()

	// Connect to mongodb
	clientOptions := options.Client().ApplyURI(hostName)
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		checkError(err)
	}

	// Verify client can connect to db
	err = client.Ping(ctx, readpref.Primary())

	if err != nil {
		checkError(err)
	}

	// 
	collection = client.Database(dbName).Collection(collectionName)
}


func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkError(err)
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	// simple validation
	if t.Title == "" {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Title filed is required",
		})
		return
	}

	// if input is okay, create a todo
	tm := todoModel{
		Id: primitive.NewObjectID(),
		Title: t.Title,
		CreatedAt: time.Now(),
		Completed: false,
	}

	_, err := collection.InsertOne(ctx, tm)

	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to save todo",
			"error": err,
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo created successfully",
		"todo_id": tm.Id.Hex(),
	})
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {

	var todos = []*todoModel{}
	// filter := bson.D{}

	curr, err := collection.Find(ctx, bson.M{})


	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"messgae": "Failed to fetch todos",
			"error": err,
		})
		return
	}

	for curr.Next(ctx) {
		var t = todoModel{}
		err := curr.Decode(&t)

		if err != nil {
			rnd.JSON(w, http.StatusProcessing, renderer.M{
				"message": "error in decodeing todos",
				"error": err,
			})
			return
		}
		todos = append(todos, &t)
	}

	if curr.Err(); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"messgae": "error in curr",
			"error": err,
		})
		return
	}

	curr.Close(ctx)

	if len(todos) == 0 {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"messgae": "No todo's in list",
		})
		return
	}

	// todos, err := filterTodos(filter)
	// if err != nil {
	// 	rnd.JSON(w, http.StatusProcessing, renderer.M{
	// 		"messge": "Failed to fetch todos",
	// 		"error": err,
	// 	})
	// 	return
	// }

	todoList := []todo{}
	for _, t := range todos {
		todoList = append(todoList, todo{
			Id: t.Id.Hex(),
			Title: t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	})

}

func filterTodos(filter interface{}) ([]*todoModel, error) {
	var todos = []*todoModel{}

	cur, err := collection.Find(ctx, filter)

	if err != nil {
		return todos, err
	}

	for cur.Next(ctx) {
		var t todoModel
		err := cur.Decode(&t)
		if err != nil {
			return todos, err
		}
		todos = append(todos, &t)
	}

	if err := cur.Err(); err != nil {
		return todos, err
	}

	cur.Close(ctx)

	if len(todos) == 0 {
		return todos, mongo.ErrNoDocuments
	}
	return todos, nil
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))


	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "The id is invalid",
		})
		return
	}

	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusProcessing, err)
		return
	}

	// simple validation
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "The title field is requried",
		})
		return
	}

	obj_id, _ := primitive.ObjectIDFromHex(id)
	
	filter := bson.D{primitive.E{Key: "_id", Value: obj_id}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "_id", Value: obj_id},
		primitive.E{Key: "title", Value: t.Title},
		primitive.E{Key: "completed", Value: t.Completed},
	}}}

	result := collection.FindOneAndUpdate(ctx, filter, update)
	if result.Err() != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to update todo",
			"error":   result.Err(),
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo updated successfully",
	})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "The id is invalid",
		})
		return
	}

	obj_id, _ := primitive.ObjectIDFromHex(id)

	filter := bson.D{primitive.E{Key: "_id", Value: obj_id}}

	_, err := collection.DeleteOne(ctx, filter)
	
	if err != nil {
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to delete todo",
			"error":   err,
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})

}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)


	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)

	r.Mount("/todo", todoHandlers())

	srv := &http.Server{
		Addr: port,
		Handler: r,
		ReadTimeout: 60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("Listning on Port: %s", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Listen error %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting Down Server...")
	// client.Disconnect(ctx)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	srv.Shutdown(ctx)
	defer cancel()
	log.Println("Server gracefully stopped!")
}


func todoHandlers() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router){
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err) // respond with error message
	}
}





