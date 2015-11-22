package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

/*Struct for getting response from user*/
type PlanReq struct {
	StartLocation string   `json:"starting_from_location_id"`
	LocationIds   []string `json:"location_ids"`
}

/*Struct for sending response*/
type PlanResp struct {
	Id               bson.ObjectId `json:"id" bson:"_id"`
	Status           string        `json:"status"`
	StartingLocation string        `json:"starting_from_location_id"`
	BestRoute        []string      `json:"best_route_location_ids"`
	TotalCost        float64       `json:"total_uber_costs"`
	TotalDuration    float64       `json:"total_uber_duration"`
	TotalDistance    float64       `json:"total_distance"`
}

//Struct for storing trip plan in DB
type PlanObj struct {
	Id               bson.ObjectId `json:"id" bson:"_id"`
	Status           string        `json:"status"`
	StartingLocation string        `json:"starting_from_location_id"`
	NextDestination  string        `json:"next_destination_location_id"`
	BestRoute        []string      `json:"best_route_location_ids"`
	TotalCost        float64       `json:"total_uber_costs"`
	TotalDuration    float64       `json:"total_uber_duration"`
	TotalDistance    float64       `json:"total_distance"`
	UberEta          int           `json:"uber_wait_time_eta"`
}

/*Struct to hold JSON received from Uber Price Estimates Api*/
type MyJson struct {
	Prices []struct {
		CurrencyCode         string  `json:"currency_code"`
		DisplayName          string  `json:"display_name"`
		Distance             float64 `json:"distance"`
		Duration             float64 `json:"duration"`
		Estimate             string  `json:"estimate"`
		HighEstimate         float64 `json:"high_estimate"`
		LocalizedDisplayName string  `json:"localized_display_name"`
		LowEstimate          float64 `json:"low_estimate"`
		Minimum              float64 `json:"minimum"`
		ProductID            string  `json:"product_id"`
		SurgeMultiplier      float64 `json:"surge_multiplier"`
	} `json:"prices"`
}

//Structure to obtain response from Uber Request Api
type MyUberReq struct {
	Driver          interface{} `json:"driver"`
	Eta             int         `json:"eta"`
	Location        interface{} `json:"location"`
	RequestID       string      `json:"request_id"`
	Status          string      `json:"status"`
	SurgeMultiplier interface{} `json:"surge_multiplier"`
	Vehicle         interface{} `json:"vehicle"`
}

//Structure to hold Json object for storing locatiosn
type Location struct {
	Id         bson.ObjectId `json:"id" bson:"_id"`
	Name       string        `json:"name"`
	Address    string        `json:"address"`
	City       string        `json:"city"`
	State      string        `json:"state"`
	Zip        string        `json:"zip"`
	Coordinate struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"coordinate"`
}

/*Struct to hold coordinates of two endpoints*/
type Path struct {
	StartLatitude  string `start_latitude`
	StartLongitude string `json:"start_longitude"`
	EndLatitude    string `json:"end_latitude"`
	EndLongitude   string `json:"end_longitude"`
}

/*Slice to store location ids from which best route is to be found*/
var a []string

/*Slice to store cost between i,j locations ids*/
var c [][]float64

/*Slice to store time between i,j locations ids*/
var t [][]float64

/*Slice to store distance between i,j locations ids*/
var d [][]float64

/*Variable storing cost for minumum route*/
var cost float64

/*slice to store Best Route*/
var route []int

type PlanSession struct {
	session *mgo.Session
}

// NewPlanSession provides a reference to a PlanSession with provided mongo session
func NewPlanSession(s *mgo.Session) *PlanSession {
	return &PlanSession{s}
}

//Api for calling Uber Sandbox request api
func reqUber(u Path, prod_id string) (MyUberReq, error) {

	var uberreq MyUberReq

	url := "https://sandbox-api.uber.com/v1/requests"

	s := map[string]string{
		"start_latitude":  u.StartLatitude,
		"start_longitude": u.StartLongitude,
		"end_latitude":    u.EndLatitude,
		"end_longitude":   u.EndLongitude,
		"product_id":      prod_id,
		//"product_id":      "04a497f5-380d-47f2-bf1b-ad4cfdcb51f2",
	}

	//	var jsonStr = []byte(`{"start_latitude":"37.334381","start_longitude":"-121.89432","end_latitude":"37.77703","end_longitude":"-122.419571","product_id": "04a497f5-380d-47f2-bf1b-ad4cfdcb51f2"}`)

	jsonStr, err := json.Marshal(s)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	req.Header.Set("X-Custom-Header", "myvalue")
	req.Header.Set("Authorization", "<Authorization code for oauth2.0>") 
	/*************************************
	   Note :- Authorization code is provided in email. 
       Please replace '<Authorization code for oauth2.0>' with the authorization code provided in email before testing the program.
	   Else the program wont work properly
	
	**************************************/
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return uberreq, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	//fmt.Println("response Body:", string(body))

	err = json.Unmarshal(body, &uberreq)
	if err != nil {
		return uberreq, err
	}

	return uberreq, nil
}

func getestimates(u Path) (MyJson, error) {

	//url := "https://api.uber.com/v1/estimates/price?start_latitude=37.484575&start_longitude=-122.1479242&end_latitude=37.3657553&end_longitude=-122.0241963&server_token=IPZvievp_m4K08RjpI97srhUVoIsZXFzXfzb6goM"
	sLat := u.StartLatitude
	sLon := u.StartLongitude
	dLat := u.EndLatitude
	dLon := u.EndLongitude

	str := ""
	str = "start_latitude=" + sLat + "&" + "start_longitude=" + sLon + "&" + "end_latitude=" + dLat + "&" + "end_longitude=" + dLon

	url := "https://sandbox-api.uber.com/v1/estimates/price?" + str + "&server_token=IPZvievp_m4K08RjpI97srhUVoIsZXFzXfzb6goM"

	var f MyJson

	res, err := http.Get(url)
	if err != nil {
		return f, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return f, err
	}

	err = json.Unmarshal(body, &f)
	if err != nil {
		return f, err
	}

	//fmt.Println(f)
	return f, nil

}

//Initializing slices for storing cost, distance and time between different end points
func initialize(ids []string, ps PlanSession) error {

	var f MyJson
	var err error
	u := Path{}

	l := len(ids)

	/*Initializing cost graph for size len(ids)*len(ids)*/
	c = make([][]float64, l)
	for i := range a {
		c[i] = make([]float64, l)
	}

	/*Initializing duration graph for size len(ids)*len(ids)*/
	t = make([][]float64, l)
	for i := range a {
		t[i] = make([]float64, l)
	}

	/*Initializing distance graph for size len(ids)*len(ids)*/
	d = make([][]float64, l)
	for i := range a {
		d[i] = make([]float64, l)
	}

	fmt.Println()
	fmt.Println("fetching data from Uber Price Estimates Api......")
	for i := 0; i < l; i++ {
		for j := 0; j < l; j++ {
			u, err = fillpath(a[i], a[j], ps)
			f, err = getestimates(u)
			if err != nil {
				return err
			}
			c[i][j] = f.Prices[0].LowEstimate
			d[i][j] = f.Prices[0].Distance
			t[i][j] = f.Prices[0].Duration
		}
	}

	//printing cost slice
	/*	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			fmt.Printf("%.f ", c[i][j])
		}
		fmt.Println()
	}*/

	return nil
}

//Function to find coorodinates of two endpoints represented with two strings ids
func fillpath(x string, y string, ps PlanSession) (Path, error) {

	var err error
	u := Path{}

	os := bson.ObjectIdHex(x)
	oe := bson.ObjectIdHex(y)

	s := Location{}
	e := Location{}

	if err := ps.session.DB("locations").C("places").FindId(os).One(&s); err != nil {
		return u, err
	}

	if err := ps.session.DB("locations").C("places").FindId(oe).One(&e); err != nil {
		return u, err
	}

	u.StartLatitude = fmt.Sprintf("%.6f", s.Coordinate.Lat)
	u.StartLongitude = fmt.Sprintf("%.6f", s.Coordinate.Lng)
	u.EndLatitude = fmt.Sprintf("%.6f", e.Coordinate.Lat)
	u.EndLongitude = fmt.Sprintf("%.6f", e.Coordinate.Lng)

	return u, err
}

//Function to swap two values
func swap(x int, y int) (int, int) {

	var temp int

	temp = x
	x = y
	y = temp

	return x, y
}

//Function for checking cose of a particular sequence
func copyArray(b []int, n int) {

	var i int
	sum := 0.0

	var sol []int
	sol = make([]int, len(b))

	for i = 0; i <= n; i++ {
		sol[i] = b[i]
		sum += c[b[i%(n+1)]][b[(i+1)%(n+1)]]
	}

	//fmt.Println("cost : ", cost, "\t sum :", sum)
	if cost > sum {
		route = sol
		cost = sum
	}
}

//Function to call all permutations of elements of the array
func computeRoute(b []int, i int, n int) {

	var j int

	if i == n {
		copyArray(b, n)
	} else {
		for j = i; j <= n; j++ {
			b[i], b[j] = swap((b[i]), (b[j]))
			computeRoute(b, i+1, n)
			b[i], b[j] = swap((b[i]), (b[j]))
		}
	}
}

//Function to Find and Add Best Route in DB
func (ps PlanSession) CreatePlan(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	fmt.Println()
	fmt.Println("creating plan.....")
	// Stub location to be populated from the body
	u := PlanReq{}

	// Populate the location data
	json.NewDecoder(r.Body).Decode(&u)

	if !bson.IsObjectIdHex(u.StartLocation) {
		w.WriteHeader(404)
		return
	}

	a = make([]string, len(u.LocationIds)+1)
	a[0] = u.StartLocation

	for i := 1; i < len(a); i++ {
		if !bson.IsObjectIdHex(u.LocationIds[i-1]) {
			w.WriteHeader(404)
			return
		}
		a[i] = u.LocationIds[i-1]
	}

	/*	for i := range a {
		fmt.Println(a[i])
	}*/

	err := initialize(a, ps)

	if err != nil {
		fmt.Println(err)
	}

	cost = 10000000
	route = make([]int, len(a))

	b := make([]int, len(a))

	for i := 0; i < len(b); i++ {
		b[i] = i
	}

	computeRoute(b, 0, len(b)-1)

	//fmt.Println("cost :", cost)
	fmt.Println()
	fmt.Println("Best Route : ")
	for _, i := range route {
		fmt.Println(i, "\t", a[i])
	}

	time := 0.0
	dist := 0.0

	for _, i := range route {
		time += t[route[i%len(route)]][route[(i+1)%len(route)]]
		dist += d[route[i%len(route)]][route[(i+1)%len(route)]]
	}

	//fmt.Println("duration :", time)
	//fmt.Println("distance :", dist)

	/*Creating Object for trip plan*/
	resp := PlanResp{}
	po := PlanObj{}

	resp.Id = bson.NewObjectId()
	resp.Status = "planning"
	resp.StartingLocation = u.StartLocation

	po.BestRoute = make([]string, len(u.LocationIds))
	resp.BestRoute = make([]string, len(u.LocationIds))
	for i := 0; i < len(resp.BestRoute); i++ {
		resp.BestRoute[i] = a[route[i+1]]
		po.BestRoute[i] = a[route[i+1]]
	}

	resp.TotalCost = cost
	resp.TotalDuration = time
	resp.TotalDistance = float64(int(dist*100)) / 100

	po.Id = resp.Id
	po.Status = resp.Status
	po.StartingLocation = resp.StartingLocation
	po.TotalCost = resp.TotalCost
	po.TotalDuration = resp.TotalDuration
	po.TotalDistance = resp.TotalDistance
	po.UberEta = 0
	po.NextDestination = ""

	//fmt.Println(resp)
	//fmt.Println()
	//fmt.Println(po)

	// Write the routes to mongo
	ps.session.DB("locations").C("routes").Insert(po)

	// Marshal provided interface into JSON structure*/
	uj, _ := json.Marshal(resp)

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)
}

// ReadLocation retrieves an individual plan resource
func (ps PlanSession) ReadPlan(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	// Grab id
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	fmt.Println()
	fmt.Println("reading plan.....")

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Stub PlanResp
	po := PlanObj{}

	// Fetch PlanResp
	if err := ps.session.DB("locations").C("routes").FindId(oid).One(&po); err != nil {
		w.WriteHeader(404)
		return
	}

	var uj []byte
	var err error

	fmt.Println()

	if po.NextDestination == "" {
		//	fmt.Println("Null Next Destination")

		u := PlanResp{}
		u.Id = po.Id
		u.StartingLocation = po.StartingLocation
		u.BestRoute = po.BestRoute
		u.TotalCost = po.TotalCost
		u.TotalDistance = po.TotalDistance
		u.TotalDuration = po.TotalDuration
		u.Status = po.Status

		uj, err = json.Marshal(u)

	} else {
		uj, err = json.Marshal(po)
	}

	if err != nil {
		fmt.Println(err)
	}

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	fmt.Fprintf(w, "%s", uj)
}

//Updates Plan Request and sets new status of the request
func (ps PlanSession) UpdatePlan(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	fmt.Println()
	fmt.Println("updating plan.....")

	// Stub a plan to be populated from the body
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Stub location
	po := PlanObj{}

	if err := ps.session.DB("locations").C("routes").FindId(oid).One(&po); err != nil {
		w.WriteHeader(404)
		return
	}

	var uj []byte
	var err error
	var f MyJson
	var nextLocation string
	u := Path{}
	presp := PlanResp{}
	uberreq := MyUberReq{}
	var i int

	fmt.Println()
	//fmt.Println("object from db")
	//fmt.Println(po)

	if po.Status == "finished" {

		fmt.Println("trip finished......")
		presp.Status = po.Status
		presp.Id = po.Id
		presp.BestRoute = po.BestRoute
		presp.StartingLocation = po.StartingLocation
		presp.TotalCost = po.TotalCost
		presp.TotalDistance = po.TotalDistance
		presp.TotalDuration = po.TotalDuration

		uj, err = json.Marshal(presp)

	} else if po.StartingLocation == po.NextDestination {

		fmt.Println("trip finished......")
		po.Status = "finished"
		po.NextDestination = ""
		po.UberEta = 0

		if err := ps.session.DB("locations").C("routes").UpdateId(oid, po); err != nil {
			w.WriteHeader(404)
			return
		}

		presp.Status = po.Status
		presp.Id = po.Id
		presp.BestRoute = po.BestRoute
		presp.StartingLocation = po.StartingLocation
		presp.TotalCost = po.TotalCost
		presp.TotalDistance = po.TotalDistance
		presp.TotalDuration = po.TotalDuration

		uj, err = json.Marshal(presp)

	} else if po.Status == "planning" {

		fmt.Println("fetching eta from Uber Requests Api......")
		u, err = fillpath(po.StartingLocation, po.BestRoute[0], ps)
		f, err = getestimates(u)
		uberreq, err = reqUber(u, f.Prices[0].ProductID)
		//fmt.Println(uberreq.Eta)

		po.UberEta = uberreq.Eta
		po.NextDestination = po.BestRoute[0]
		po.Status = "requesting"

		if err := ps.session.DB("locations").C("routes").UpdateId(oid, po); err != nil {
			w.WriteHeader(404)
			return
		}

		uj, err = json.Marshal(po)

	} else {

		fmt.Println("fetching eta from Uber Requests Api......")
		for i = 0; i < len(po.BestRoute); i++ {
			if po.BestRoute[i] == po.NextDestination {
				break
			}
		}

		//fmt.Println(i)
		//fmt.Println(po.BestRoute[i])

		if i == (len(po.BestRoute) - 1) {
			nextLocation = po.StartingLocation
		} else {
			nextLocation = po.BestRoute[i+1]
		}

		u, err = fillpath(po.BestRoute[i], nextLocation, ps)
		f, err = getestimates(u)
		uberreq, err = reqUber(u, f.Prices[0].ProductID)
		//fmt.Println(uberreq.Eta)

		po.UberEta = uberreq.Eta
		po.NextDestination = nextLocation

		if err := ps.session.DB("locations").C("routes").UpdateId(oid, po); err != nil {
			w.WriteHeader(404)
			return
		}

		uj, err = json.Marshal(po)

	}

	if err != nil {
		w.WriteHeader(404)
		return
	}

	// Write content-type, statuscode, payload
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	fmt.Fprintf(w, "%s", uj)
}

//Function to delete any idea in db
func (ps PlanSession) DeletePlan(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	// Grab id
	id := p.ByName("id")

	// Verify id is ObjectId, otherwise bail
	if !bson.IsObjectIdHex(id) {
		w.WriteHeader(404)
		return
	}

	// Grab id
	oid := bson.ObjectIdHex(id)

	// Remove user
	if err := ps.session.DB("locations").C("routes").RemoveId(oid); err != nil {
		w.WriteHeader(404)
		return
	}

	// Write status
	w.WriteHeader(200)
}

//This function creates mongosession or panics if error occurs
func getSession() *mgo.Session {

	// Connect to mongodb
	s, err := mgo.Dial("mongodb://dbuserShubhra:shubhra123@ds045054.mongolab.com:45054/locations")

	// Check if connection error, is mongo running?
	if err != nil {
		panic(err)
	}

	// Deliver session
	return s
}

func main() {
	// Instantiate a new router
	r := httprouter.New()

	//Creating new Plan trip session with mgosession
	nls := NewPlanSession(getSession())

	// Add a handler
	r.POST("/trips", nls.CreatePlan)
	r.GET("/trips/:id", nls.ReadPlan)
	r.PUT("/trips/:id/request", nls.UpdatePlan)
	r.DELETE("/trips/:id", nls.DeletePlan)

	// Fire up the server
	http.ListenAndServe("localhost:3000", r)
}
