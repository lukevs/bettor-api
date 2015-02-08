package main

import (
    "database/sql"
    "errors"
    "fmt"
    "math/rand"
    "net/http"
    "net/url"
    "os"
    "strconv"
    "strings"

    _ "github.com/go-sql-driver/mysql"
)

// A User represents basic info about a user.
type User struct {
    Id int                  `json:"id"`
    FirstName string        `json:"first_name"`
    LastName string         `json:"last_name"`
    Email string            `json:"email"`
    AccessToken string      `json:"access_token"`
    ProfilePicUrl string    `json:"profile_pic_url"`
    CreatedOn time.Time     `json:"created_on"`
    VenmoId string          `json:"venmo_id"`
}

// Sends a text message with a user's verification token so they can confirm their phone number.
func SendVerificationMsg(accessToken string, phoneNumber string) error{
    verificationToken, _ := GetVerificationTokenFromAccessToken(accessToken)
    msg := fmt.Sprintf("Your Bettor verification id is: %s", verificationToken)
    SendTwilioMsg(phoneNumber, msg)
}

// Sends a text message from the Twilio API.
func SendTwilioMsg(phoneNumber string, message string) error{
    accountSid := "AC4b7b097d333a0d6490fff5d1098db453"
    authToken := os.Getenv("TWILIO_SECRET_KEY")
    urlString := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSid)

    url_params := url.Values{}
    url_params.Set("From", "+19782212680")
    url_params.Set("To", fmt.Sprintf("+1%s", phoneNumber))
    url_params.Set("Body", message)
    req_body := *strings.NewReader(url_params.Encode())

    client := &http.Client{}
    req, err := http.NewRequest("POST", urlString, &req_body)
    if err != nil{
        return errors.New("There was an error creating your NewRequest: " + err.Error())
    }
    req.SetBasicAuth(accountSid, authToken)
    req.Header.Add("Accept", "application/json")
    req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

    resp, err := client.Do(req)
    if err != nil{
        return errors.New("The verification text message has failed to send: " + err.Error())
    }
}

// CreateUser creates a new user.
func (db *sql.DB) CreateUser(firstName string,
                             lastName string,
                             email string,
                             accessToken string,
                             profilePicUrl string,
                             venmoId string) error {

    if VenmoUserExists(venmoId) {
        return errors.New("A user already exists for the given Venmo id")
    }

    var verificationToken string
    for i := 0; i < 4; i++ {
        verification_token += strconv.Itoa(rand.Intn(10))
    }

    q := "insert into users (first_name, last_name, email, " +
             "access_token, verification_token, profile_pic_url, venmo_id) values (?)"

    stmt, err := db.Prepare(q)
    if err != nil {
        return errors.New("Failed to prepare user insert: " + err.Error())
    }
    defer stmt.Close()

    values := fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s",
                          firstName,
                          lastName,
                          email,
                          accessToken,
                          verificationToken,
                          profilePicUrl,
                          venmoId)

    _, err = stmt.Exec(values)
    if err != nil {
        return errors.new("Failed to execute user insert: " + err.Error())
    }

    return nil
}

// DeleteUser deletes a user.
func (db *sql.DB) DeleteUser(id int) error {

    _, err := db.Exec("update users set is_deleted = 1 where id = ?", id)
    if err != nil {
        return errors.New("Failed to delete user: " + err.Error())
    }

    return nil
}

// UpdateUser updates information about a user.
// If there is a phone number passed in, we also verify their phone number.
func (db *sql.DB) UpdateUser(id int, args map[string]string) error {
    if _, ok := args['phone_number']; ok {
        SendVerificationMsg(args['phone_number'])
    }

    statement := "update users set "
    for k, v in range args {
        statement += (k + "=" + v + ",")
    }

    // remove the last comma
    statement = statement[:len(statement) - 1]
    statement += "where id = ?"

    stmt, err := db.Prepare(statement)
    if err != nil {
        return errors.New("Failed to prepare user update: " + err.Error())
    }

    _, err := stmt.Exec(id)
    if err != nil {
        return errors.New("Failed to execute user update: " + err.Error())
    }

    return nil
}

// GetUser returns a User reflecting the current state of a given user.
func (db *sql.DB) GetUser(id int) (*User, error) {

    var u User
    q := "select id, first_name, last_name, email, " +
             "access_token, profile_pic_url, created_on," +
             " venmo_id from users where id = ?"

    err := db.QueryRow(q, id).Scan(&u.Id,
                                       &u.FirstName,
                                       &u.LastName,
                                       &u.Email,
                                       &u.AccessToken,
                                       &u.ProfilePicUrl,
                                       &u.CreatedOn,
                                       &u.VenmoId)
    if err != nil {
        return nil, errors.New("Failed to get user: " + err.Error())
    }

    return &u, nil
}

// GetUsers returns a slice of Users matchign the given arguments.
func (db *sql.DB) GetUsers(args map[string]string)) ([]User, error) {

    var u User
    users := make([]User)

    q := "select id, first_name, last_name, email, " +
             "access_token, profile_pic_url, created_on," +
             " venmo_id from users where is_deleted = 0 and is_verified = 1"

    for k, v := range args {
        q += (k + "=" v " and ")
    }

    q = q[:len(q) - 5]

    rows, err := db.Query(q)
    if err != nil {
        return nil, errors.New("Failed query for users: " + err.Error())
    }
    defer rows.Close()

    for rows.Next() {
        err := rows.Scan(&u.Id,
                         &u.FirstName,
                         &u.LastName,
                         &u.Email,
                         &u.AccessToken,
                         &u.ProfilePicUrl,
                         &u.CreatedOn,
                         &u.VenmoId)
        if err != nil {
            return nil, errors.New("Failed to scan user row: " + err.Error())
        }

        users.append(u)
    }

    err = rows.Err()
    if err != nil {
        return nil, errors.New("Failed while iterating over user rows: " + err.Error())
    }

    return users[0:], nil
}

// GetUserBets gets the bets for a given user.
func (db *sql.DB) GetUserBets(id int) ([]Bet, error) {

    var b Bet
    bets := make([]Bet)

    q := "select id, bettor_id, betted_id, witness_id, " +
         "winner_id, title, desc, created_at, expire_at, " +
         "status, amount from bets where (bettor_id = ? or betted_id = ?)"

    rows, err := db.Query(q, id, id)
    if err != nil {
        return nil, errors.New("Failed query for user bets: " + err.Error())
    }
    defer rows.Close()

    for rows.Next() {
        err := rows.Scan(&b.Id,
                         &b.BettorId,
                         &b.BettedId,
                         &b.WitnessId,
                         &b.WinnerId,
                         &b.Title,
                         &b.Desc,
                         &b.CreatedAt,
                         &b.ExpireAt,
                         &b.Status,
                         &b.Amount)
        if err != nil {
            return nil, errors.New("Failed to scan user row: " + err.Error())
        }

        bets.append(b)
    }

    err = rows.Err()
    if err != nil {
        return nil, errors.New("Failed while iterating over user bet rows: " + err.Error())
    }

    return users[0:], nil
}

// GetUserWitnessing gets the bets for which a user is a witness.
func (db *sql.DB) GetUserWitnessing(id int) ([]Bet, error) {
    var b Bet
    bets := make([]Bet)

    q := "select id, bettor_id, betted_id, witness_id, " +
         "winner_id, title, desc, created_at, expire_at, " +
         "status, amount from bets where witness_id = ?)"

    rows, err := db.Query(q, id)
    if err != nil {
        return nil, errors.New("Failed query for user bets: " + err.Error())
    }
    defer rows.Close()

    for rows.Next() {
        err := rows.Scan(&b.Id,
                         &b.BettorId,
                         &b.BettedId,
                         &b.WitnessId,
                         &b.WinnerId,
                         &b.Title,
                         &b.Desc,
                         &b.CreatedAt,
                         &b.ExpireAt,
                         &b.Status,
                         &b.Amount)
        if err != nil {
            return nil, errors.New("Failed to scan user witnesses: " + err.Error())
        }

        bets.append(b)
    }

    err = rows.Err()
    if err != nil {
        return nil, errors.New("Failed while iterating over user witness rows: " + err.Error())
    }

    return bets[0:], nil
}

// UserExists checks if a user with the given id exists.
func (db *sql.DB) UserExists(id int) {
   var tmp int
   err := db.QueryRow("select id from users where id = ?", id).Scan(&tmp)
   return err == sql.ErrNoRows
}

// VenmoUserExists checks if a user already exists using a Venmo id.
func (db *sql.DB) VenmoUserExists(venmoId string) bool {

    // there has to be a better way to get the errors from QueryRow
    var id int
    err := db.QueryRow("select id from users where venmo_id = ?", venmoId).Scan(&id)
    return err == sql.ErrNoRows
}

// Returns the verification token based on a given user's Venmo access token.
func (db *sql.DB) GetVerificationTokenFromAccessToken(accessToken string) (string, error){
    var verificationToken string
    err := db.QueryRow("select verification_token from users where access_token = ?", accessToken).Scan(&verification_token)
    if err != nil{
        return nil, errors.New("Failed while querying for the verification token: " + err.Error())
    }
    return verificationToken, nil
}

// Sets is_verified on a given user to True when we verify their phone number.
func (db *sql.DB) VerifyUser(accessToken string, verificationToken string) error{
    dBVerificationToken, _ = GetVerificationTokenFromAccessToken(accessToken)

    if dBVerificationToken != verificationToken {
        return errors.New("Your access token does not match our records. Try again?")
    }

    _, err := db.Exec("update users set is_verified = 1 where access_token = ?", aceess_token)
    if err != nil{
        return errors.New("Error when setting is_verified for the current user: " + err.Error())
    }

    return nil
}