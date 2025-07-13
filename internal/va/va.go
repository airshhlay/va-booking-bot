package va

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/airshhlay/va-booking-bot/internal/auth"
	"github.com/airshhlay/va-booking-bot/internal/util"
)

var (
	ErrClassNotFound         = errors.New("class not found")
	ErrNoMoreSpaceInClass    = errors.New("no more space in class")
	ErrNoMoreSeats           = errors.New("no more seats")
	ErrUserHasNoMoreBookings = errors.New("user has no more bookings")
	ErrBookingNotSuccess     = errors.New("booking not success")
	ErrUserCannotBook        = errors.New("user can not book")
)

var (
	_client      = &http.Client{}
	_token       token
	_tokenExpiry time.Time
)

var (
	errFailedReq = errors.New("failed req")
)

const (
	_origin   = "https://mylocker.virginactive.com.sg"
	_referrer = "https://mylocker.virginactive.com.sg"

	_classTimeFormat = "2006-01-02T15:04:05"
	_isoDateFormat   = "2006-01-02"
)

type SiteID string

const (
	SiteIDPayaLebar    SiteID = "SPL"
	SiteIDRafflesPlace SiteID = "SRP"
)

type ClassName string

const (
	ClassNameCycleSpirit ClassName = "Cycle - Spirit"
)

type token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	MemberID    string `json:"member_id"`
}

func GetToken(ctx context.Context) (token, error) {
	const (
		_vaTokenUrl = "https://hal.virginactive.com.sg/token"
	)

	currTime := time.Now()

	if _tokenExpiry.After(currTime) {
		return _token, nil
	}

	// post request to get token
	form := url.Values{}
	form.Set("username", auth.GetUserName())
	form.Set("password", auth.GetPassword())
	form.Set("grant_type", "password")

	req, err := http.NewRequest("POST", _vaTokenUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return token{}, err
	}
	setHeaders(req)

	resp, err := _client.Do(req)
	if err != nil {
		return token{}, err
	}

	if resp.StatusCode != 200 {
		log.Fatal(resp.Body)
		return token{}, errFailedReq
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	res := token{}
	if err := decoder.Decode(&res); err != nil {
		return token{}, err
	}

	_token = res
	_tokenExpiry = currTime.Add(time.Second*time.Duration(res.ExpiresIn) - time.Hour*1) // some buffer
	return res, nil
}

type getBookableClassBody struct {
	Category int    `json:"Category"`
	AMPM     string `json:"AMPM"`
	ISODate  string `json:"ISODate"`
	SiteID   SiteID `json:"SiteID"`
}

type vaClass struct {
	TypeInd         int
	BookingID       int
	StartDateTime   string
	EndDateTime     string
	ClassName       string
	Instructor      string
	SpacesRemaining int
	Plus2Identifier string
}

type getBookableClassResp struct {
	Classes []vaClass
}

type getClassParams struct {
	classDate time.Time
	className ClassName
}

func GetClass(ctx context.Context, params getClassParams) (vaClass, error) {
	const (
		_endpoint = "https://hal.virginactive.com.sg/api/classes/bookableclassquery"
	)
	token, err := GetToken(ctx)
	if err != nil {
		return vaClass{}, err
	}

	body := getBookableClassBody{
		Category: 0,
		AMPM:     "ALL",
		ISODate:  params.classDate.Format(_isoDateFormat),
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return vaClass{}, err
	}

	req, err := http.NewRequest("POST", _endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return vaClass{}, nil
	}
	setHeaders(req)
	setAuthorisation(req, token)

	resp, err := _client.Do(req)
	if err != nil {
		return vaClass{}, err
	}

	if resp.StatusCode != 200 {
		log.Fatal(resp.Body)
		return vaClass{}, errFailedReq
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	res := getBookableClassResp{}
	if err := decoder.Decode(&res); err != nil {
		return vaClass{}, err
	}

	log.Println(util.MarshaltoString(ctx, res))

	for _, class := range res.Classes {
		if strings.Contains(class.ClassName, string(params.className)) && class.StartDateTime == params.classDate.Format(_classTimeFormat) {
			return class, nil
		}
	}

	return vaClass{}, ErrClassNotFound
}

type BookClassParams struct {
	SiteID          SiteID
	ClassTime24Hour string
	ClassDay        int
	ClassName       ClassName
	Instructor      string
	SeatPriorities  []int
}

type bookClassBody struct {
	MemberID   int
	BookingID  int
	SeatNumber int
	Message    string
}

type bookClassResp struct {
	Success bool
}

func BookClass(ctx context.Context, params BookClassParams) error {
	const (
		_endpoint = "https://hal.virginactive.com.sg/api/bookings/makeclassbooking"
	)

	wantClassDate, err := util.GetNextScheduledTime(params.ClassDay, params.ClassTime24Hour)
	if err != nil {
		return err
	}
	class, err := GetClass(ctx, getClassParams{
		classDate: wantClassDate,
		className: params.ClassName,
	})
	if err != nil {
		return err
	}
	if class.SpacesRemaining == 0 {
		return ErrNoMoreSpaceInClass
	}

	// get class seats
	availSeats, err := GetAvailableClassSeats(ctx, class.BookingID, class.Plus2Identifier)
	if err != nil {
		return err
	}
	if len(availSeats) == 0 {
		return ErrNoMoreSeats
	}

	seatMap := make(map[int]bool, len(availSeats))
	for _, seat := range availSeats {
		seatMap[seat.SeatNumber] = true
	}

	var toBook int
	for _, priority := range params.SeatPriorities {
		if seatMap[priority] {
			// book this seat
			toBook = priority
			break
		}
	}
	if toBook == 0 {
		// pick a random avail seat
		toBook = availSeats[0].SeatNumber
	}

	token, err := GetToken(ctx)
	if err != nil {
		return err
	}

	memberID, err := strconv.ParseInt(token.MemberID, 10, 32)
	if err != nil {
		return err
	}
	body := bookClassBody{
		MemberID:   int(memberID),
		BookingID:  class.BookingID,
		SeatNumber: toBook,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", _endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	setHeaders(req)
	setAuthorisation(req, token)

	resp, err := _client.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		log.Fatal(resp.Body)
		return errFailedReq
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	res := bookClassResp{}
	if err := decoder.Decode(&res); err != nil {
		return err
	}

	if !res.Success {
		return ErrBookingNotSuccess
	}

	return nil
}

type seat struct {
	RoomItemType int
	SeatNumber   int
}

type getClassSeatsBody struct {
	BookingID                 int
	Plus2DescriptionProductID string
}

type getClassSeatsResp struct {
	RemainingBookingsCount int
	RoomLayout             [][]seat
	CanIBook               int
}

func GetAvailableClassSeats(ctx context.Context, bookingID int, productID string) ([]seat, error) {
	const (
		_endpoint              = "https://hal.virginactive.com.sg/api/classes/getclassoptions"
		_availableRoomItemType = 1
	)

	token, err := GetToken(ctx)
	if err != nil {
		return nil, nil
	}

	body := getClassSeatsBody{
		BookingID:                 bookingID,
		Plus2DescriptionProductID: productID,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", _endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, nil
	}
	setHeaders(req)
	setAuthorisation(req, token)

	resp, err := _client.Do(req)
	if err != nil {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		log.Fatal(resp.Body)
		return nil, errFailedReq
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	res := getClassSeatsResp{}
	if err := decoder.Decode(&res); err != nil {
		return nil, err
	}

	if res.CanIBook == 1 {
		return nil, ErrUserCannotBook
	}

	var availableSeats []seat
	for _, row := range res.RoomLayout {
		for _, seat := range row {
			if seat.RoomItemType == _availableRoomItemType {
				availableSeats = append(availableSeats, seat)
			}
		}
	}

	return availableSeats, nil
}

func setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8,zh-CN;q=0.7,zh;q=0.6")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("DNT", "1")
	req.Header.Set("Origin", _origin)
	req.Header.Set("Referer", _referrer)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"macOS"`)
	req.Header.Set("x-mylocker-language", "en-SG")
}

func setAuthorisation(req *http.Request, token token) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
}
