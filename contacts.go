package main

import (
    "database/sql"
    "errors"
    "fmt"
    "time"

    _ "github.com/go-sql-driver/mysql"
)

// A Contact pair connects a phone number with an associated user id.
type ContactPair struct{
    PhoneNumber string              `json:"phone_number"`
    UserId int                      `json:"user_id"`
}

func (db *sql.DB) CheckPhoneNumbers(phone_numbers []string) ([]ContactPair, error){
    contactpairs := make([]ContactPair)
    var cp ContactPair

    query := "select id, phone_number from users where phone_number in ?"

    for phone_number := range phone_numbers{
        query += fmt.Sprintf("%s, " phone_number)
    }
    query := query[:-2]

    stmt, err := db.Prepare(query)
    if err != nil{
        return errors.New("Error when preparing the CheckPhoneNumbers query")
    }
    defer stmt.Close()

    rows, err := stmt.Query()
    if err != nil{
        return errors.New("Error when executing the CheckPhoneNumbers query")
    }
    defer rows.Close()


    for rows.Next(){
        err = rows.Scan(&cp.PhoneNumber, &cp.UserId)
        contactpairs.append(cp)
    }
    if err = rows.Err(); err != nil{
        return errors.New("Error when scanning the CheckPhoneNumbers query")
    }

    return contactpairs, nil
}