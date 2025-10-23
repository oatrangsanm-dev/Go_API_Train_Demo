package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type TrainInfo struct {
	TripID          string `json: "trip_Id"`
	TrainNo         string `json: "train_No"`
	TrainTypeNameTh string `json: "train_Type_Name_Th"`
	OriginStationNo string `json: "origin_Station_No"`
	DestStationNo   string `json: "dest_Station_No"`
	TripDate        string `json: "trip_Date"`
	OriginTime      string `json: "origin_Time"`
	DestTime        string `json: "dest_Time"`
	TotalDistance   string `json: "total_Distance"`
}

var Db *sql.DB
var TrainList []TrainInfo

const basePath = "/api"
const trainPath = "train"

func getTrainTrip(trip_id string) ([]TrainInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var rows *sql.Rows
	var err error

	if trip_id == "" {
		rows, err = Db.QueryContext(ctx, `
			SELECT trip_id, train_no, train_type_name_th, origin_station_no,
			       dest_station_no, trip_date, origin_time, dest_time, total_distance
			FROM train_trip
		`)
	} else {
		rows, err = Db.QueryContext(ctx, `
			SELECT trip_id, train_no, train_type_name_th, origin_station_no,
			       dest_station_no, trip_date, origin_time, dest_time, total_distance
			FROM train_trip
			WHERE trip_id = ?
		`, trip_id)
	}

	if err != nil {
		log.Println("query error:", err)
		return nil, err
	}
	defer rows.Close()

	var trainTrips []TrainInfo

	for rows.Next() {
		var traininfo TrainInfo
		err := rows.Scan(
			&traininfo.TripID,
			&traininfo.TrainNo,
			&traininfo.TrainTypeNameTh,
			&traininfo.OriginStationNo,
			&traininfo.DestStationNo,
			&traininfo.TripDate,
			&traininfo.OriginTime,
			&traininfo.DestTime,
			&traininfo.TotalDistance,
		)
		if err != nil {
			log.Println("scan error:", err)
			return nil, err
		}
		trainTrips = append(trainTrips, traininfo)
	}

	if err = rows.Err(); err != nil {
		log.Println("rows error:", err)
		return nil, err
	}

	return trainTrips, nil
}

func handleTrains(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tripID := r.URL.Query().Get("trip_id")
		trainTrips, err := getTrainTrip(tripID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		j, err := json.Marshal(trainTrips)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(j)

	case http.MethodPost:
		var traininfo TrainInfo
		err := json.NewDecoder(r.Body).Decode(&traininfo)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		Train_ID, err := insertTrain(traininfo)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(fmt.Sprintf(`{"trip_id":%d}`, Train_ID)))

	case http.MethodPut:
		var traininfo TrainInfo
		err := json.NewDecoder(r.Body).Decode(&traininfo)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rows, err := updateTrain(traininfo)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if rows == 0 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"ไม่พบขบวนรถไฟที่ต้องการแก้ไขข้อมูล"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"อัพเดตขบวนรถไฟสำเร็จ"}`))

	case http.MethodOptions:
		return

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTrainTrip(w http.ResponseWriter, r *http.Request) {
	urlPathSegments := strings.Split(r.URL.Path, fmt.Sprintf("%s/", trainPath))
	if len(urlPathSegments[1:]) > 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	tripID := urlPathSegments[len(urlPathSegments)-1]
	if tripID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		traintrip, err := getTrainTrip(tripID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if traintrip == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		j, err := json.Marshal(traintrip)
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err = w.Write(j)
		if err != nil {
			log.Fatal(err)
		}

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func insertTrain(traininfo TrainInfo) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	now := time.Now()

	result, err := Db.ExecContext(ctx, `INSERT INTO go_train.train_trip
	(trip_Id,
	train_No,
	train_Type_Name_Th,
	origin_Station_No,
	dest_Station_No,
	trip_Date,
	origin_Time,
	dest_Time,
	total_Distance,
	create_date,
	update_date)
	VALUES
	(?,?,?,?,?,?,?,?,?,?,?)`,
		traininfo.TripID,
		traininfo.TrainNo,
		traininfo.TrainTypeNameTh,
		traininfo.OriginStationNo,
		traininfo.DestStationNo,
		traininfo.TripDate,
		traininfo.OriginTime,
		traininfo.DestTime,
		traininfo.TotalDistance,
		now,
		now,
	)
	if err != nil {
		log.Println(err.Error())
		return 0, err
	}
	insertTripID, err := result.LastInsertId()
	if err != nil {
		log.Println(err.Error())
		return 0, err
	}
	return int(insertTripID), nil
}

func updateTrain(traininfo TrainInfo) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	now := time.Now()

	result, err := Db.ExecContext(ctx, `
		UPDATE go_train.train_trip
		SET 
			train_No = ?,
			train_Type_Name_Th = ?,
			origin_Station_No = ?,
			dest_Station_No = ?,
			trip_Date = ?,
			origin_Time = ?,
			dest_Time = ?,
			total_Distance = ?,
			update_date = ?
		WHERE trip_Id = ?`,
		traininfo.TrainNo,
		traininfo.TrainTypeNameTh,
		traininfo.OriginStationNo,
		traininfo.DestStationNo,
		traininfo.TripDate,
		traininfo.OriginTime,
		traininfo.DestTime,
		traininfo.TotalDistance,
		now,
		traininfo.TripID,
	)
	if err != nil {
		log.Println("Update error:", err.Error())
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("RowsAffected error:", err.Error())
		return 0, err
	}

	return rowsAffected, nil
}

func corsMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Authorization, X-Custom-Header, x-requested-with")
		handler.ServeHTTP(w, r)
	})
}

func SetupRoutes(apiBasePath string) {
	trainsHandler := http.HandlerFunc(handleTrains)
	traintripHandler := http.HandlerFunc(handleTrainTrip)
	http.Handle(fmt.Sprintf("%s/%s", apiBasePath, trainPath), corsMiddleware(trainsHandler))
	http.Handle(fmt.Sprintf("%s/%s/", apiBasePath, trainPath), corsMiddleware(traintripHandler))
}

func SetupDB() {
	var err error
	Db, err = sql.Open("mysql", "root:root@tcp(127.0.0.1:3306)/go_train?parseTime=true&charset=utf8mb4&loc=Local")
	if err != nil {
		log.Fatal(err)
	}

	if err = Db.Ping(); err != nil {
		log.Fatal("Cannot connect to database:", err)
	}

	fmt.Println("Connected to MySQL successfully")

	Db.SetConnMaxLifetime(time.Minute * 3)
	Db.SetMaxOpenConns(10)
	Db.SetMaxIdleConns(10)
}

func main() {
	SetupDB()
	SetupRoutes(basePath)
	log.Fatal(http.ListenAndServe(":8000", nil))
}
