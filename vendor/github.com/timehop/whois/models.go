package whois

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/timehop/golog/log"
	"github.com/timehop/goth/lang"
	"github.com/timehop/goth/thjson"
	"github.com/timehop/goth/timehop"
)

const (
	FacebookAccountType   = "FacebookAccount"
	TwitterAccountType    = "TwitterAccount"
	InstagramAccountType  = "InstagramAccount"
	FoursquareAccountType = "FoursquareAccount"
	DropboxAccountType    = "DropboxAccount"
	GoogleAccountType     = "GoogleAccount"

	FacebookTypeKey   = "facebook"
	TwitterTypeKey    = "twitter"
	InstagramTypeKey  = "instagram"
	FoursquareTypeKey = "foursquare"
	DropboxTypeKey    = "dropbox"
	GoogleTypeKey     = "google"
)

var AllAccountTypes = []string{
	FacebookAccountType,
	TwitterAccountType,
	InstagramAccountType,
	FoursquareAccountType,
	DropboxAccountType,
	GoogleAccountType,
}

type Account struct {
	ID                    thjson.Number `json:"id"`
	UserID                thjson.Number `json:"user_id"`
	ExternalAccountID     string        `json:"external_account_id"` // Aka: `uid`
	Handle                string        `json:"handle"`
	AccessToken           string        `json:"access_token"`
	SecretToken           string        `json:"secret_token,omitempty"`
	UnauthorizedCallCount thjson.Number `json:"unauthorized_call_count"`
	Type                  string        `json:"type"`
	AccessTokenExpiration *time.Time    `json:"access_token_expiration,omitempty"`
	Settings              []string      `json:"settings"`
	DropboxFolders        []string      `json:"dropbox_folders,omitempty"` // Dropbox only...le sigh
	TwitterInfo           *TwitterInfo  `json:"twitter_info,omitempty"`    // Twitter only...le sigh

	LegacyID                thjson.Number `json:"account_id"`
	LegacyExternalAccountID string        `json:"uid"`
}

func (a *Account) IsTokenExpired() bool {
	if a.Type == GoogleAccountType {
		return false
	}
	return a.AccessTokenExpiration != nil && a.AccessTokenExpiration.Unix() < time.Now().Unix()
}

func (a *Account) IsAuthorized() bool {
	return a.UnauthorizedCallCount == 0 && !a.IsTokenExpired()
}

func (s *Account) HasSetting(v string) bool {
	for _, setting := range s.Settings {
		if strings.EqualFold(setting, v) {
			return true
		}
	}
	return false
}

// TODO Kevin S ... I think we can nuke this now since we don't have the legacy field issue?
func (a *Account) RespectLegacyFields() {
	// Handles the case where older cache values set json key "account_id" instead of "id".
	if a.ID == a.UserID {
		a.ID = a.LegacyID
	} else {
		a.LegacyID = a.ID
	}
	// Handles the case where older cache values set json key "uid" instead of "external_account_id".
	if a.ExternalAccountID == "" {
		a.ExternalAccountID = a.LegacyExternalAccountID
	} else {
		a.LegacyExternalAccountID = a.ExternalAccountID
	}
}

func (a *Account) Equals(a2 *Account) bool {
	if reflect.DeepEqual(a, a2) {
		return true
	} else if a == nil || a2 == nil {
		return false
	}

	if a.ID != a2.ID ||
		a.UserID != a2.UserID ||
		a.ExternalAccountID != a2.ExternalAccountID ||
		a.AccessToken != a2.AccessToken ||
		a.SecretToken != a2.SecretToken ||
		a.UnauthorizedCallCount != a2.UnauthorizedCallCount ||
		a.Type != a2.Type ||
		len(a.Settings) != len(a2.Settings) {
		return false
	}

	if (a.AccessTokenExpiration != nil || a2.AccessTokenExpiration != nil) && (a.AccessTokenExpiration == nil || a2.AccessTokenExpiration == nil || !a.AccessTokenExpiration.Equal(*a2.AccessTokenExpiration)) {
		return false
	}

	for i, setting := range a.Settings {
		if setting != a2.Settings[i] {
			return false
		}
	}

	return true
}

type UserAccounts struct {
	// UserID only exists to conform to cache.Value interface
	UserID   thjson.Number `json:"user_id"`
	Accounts []Account     `json:"accounts"`

	// TODO Kevin S: I think we can not remove older fields because the legacy field
	// problem no longer exists.
	// Here to support existing cache entries. These values are not
	// guaranteed to be set and in fact are unlikely to be!
	LegacyUserID   thjson.Number `json:"UserID"`
	LegacyAccounts []Account     `json:"Accounts"`
}

func (userAccounts *UserAccounts) RespectLegacyFields() {
	// Swap legacy values into new values
	if userAccounts.UserID.Int64() == 0 {
		userAccounts.UserID = userAccounts.LegacyUserID
	} else {
		userAccounts.LegacyUserID = userAccounts.UserID
	}

	userAccountsAccounts := userAccounts.Accounts
	if len(userAccountsAccounts) == 0 {
		userAccountsAccounts = userAccounts.LegacyAccounts
	}

	accounts := make([]Account, len(userAccountsAccounts))
	for i, a := range userAccountsAccounts {
		a.RespectLegacyFields()
		accounts[i] = a
	}

	userAccounts.Accounts = accounts
	userAccounts.LegacyAccounts = accounts
}

type UserField string

const (
	UserFieldAccounts  UserField = "accounts"
	UserFieldAuthToken           = "auth_token"
	UserFieldUserPrefs           = "user_prefs"
)

type SignupType string

const (
	SignupTypeFacebook SignupType = "facebook"
	SignupTypePhone    SignupType = "phone"
)

type User struct {
	// Call ID.Int64() to cast
	ID        thjson.Number `json:"id"`
	CreatedAt *time.Time    `json:"created_at,omitempty"`
	Timezone  string        `json:"time_zone,omitempty"`

	// Personal & Demographics
	FacebookUserID string  `json:"facebook_user_id"`
	FirstName      string  `json:"first_name,omitempty"`
	LastName       string  `json:"last_name,omitempty"`
	Birthdate      *Date   `json:"birthdate"`
	CountryName    string  `json:"country_name"`
	Language       string  `json:"language"`
	Gender         string  `json:"gender"`
	Email          *string `json:"email,omitempty"`
	PhoneNumber    *string `json:"phone_number,omitempty"`

	// Flags
	Admin bool `json:"admin"`
	Beta  bool `json:"beta"`

	// Misc
	LatestAppVersion        string     `json:"latest_app_version"`
	LatestAndroidAppVersion string     `json:"latest_android_app_version"`
	DownloadedOSXAppAt      *time.Time `json:"downloaded_osx_app_at,omitempty"`
	DownloadedWindowsAppAt  *time.Time `json:"downloaded_windows_app_at,omitempty"`

	SignupType SignupType `json:"signup_type"`

	// Entity relationships
	Preferences *UserPreferences `json:"user_prefs,omitempty"`
	Accounts    []Account        `json:"accounts,omitempty"`
	AuthToken   *AppAuthToken    `json:"app_auth_token,omitempty"`
}

func (u *User) Equals(u2 *User) bool {
	if reflect.DeepEqual(u, u2) {
		return true
	} else if u == nil || u2 == nil {
		return false
	}

	if u.ID != u2.ID ||
		u.Timezone != u2.Timezone ||
		u.FacebookUserID != u2.FacebookUserID ||
		u.FirstName != u2.FirstName ||
		u.LastName != u2.LastName ||
		u.CountryName != u2.CountryName ||
		u.Language != u2.Language ||
		u.Gender != u2.Gender ||
		u.Admin != u2.Admin ||
		u.Beta != u2.Beta ||
		u.LatestAppVersion != u2.LatestAppVersion ||
		u.LatestAndroidAppVersion != u2.LatestAndroidAppVersion ||
		u.SignupType != u2.SignupType ||
		!u.AuthToken.Equals(u2.AuthToken) {
		return false
	}

	if (u.CreatedAt != nil || u2.CreatedAt != nil) && (u.CreatedAt == nil || u2.CreatedAt == nil || !u.CreatedAt.Equal(*u2.CreatedAt)) {
		return false
	}

	if (u.DownloadedOSXAppAt != nil || u2.DownloadedOSXAppAt != nil) && (u.DownloadedOSXAppAt == nil || u2.DownloadedOSXAppAt == nil || !u.DownloadedOSXAppAt.Equal(*u2.DownloadedOSXAppAt)) {
		return false
	}

	if (u.DownloadedWindowsAppAt != nil || u2.DownloadedWindowsAppAt != nil) && (u.DownloadedWindowsAppAt == nil || u2.DownloadedWindowsAppAt == nil || !u.DownloadedWindowsAppAt.Equal(*u2.DownloadedWindowsAppAt)) {
		return false
	}

	if (u.Birthdate != nil || u2.Birthdate != nil) && (u.Birthdate == nil || u2.Birthdate == nil || *u.Birthdate != *u2.Birthdate) {
		return false
	}

	if (u.Preferences != nil || u2.Preferences != nil) && (u.Preferences == nil || u2.Preferences == nil || *u.Preferences != *u2.Preferences) {
		return false
	}

	if len(u.Accounts) != len(u2.Accounts) {
		return false
	}
	for i, account := range u.Accounts {
		if !account.Equals(&u2.Accounts[i]) {
			return false
		}
	}

	return true
}

func (u User) IsNew() bool {
	if u.CreatedAt == nil {
		return false
	}
	return time.Now().Sub(*u.CreatedAt) <= time.Hour
}

func (u User) AccountByType(accountType string) (found *Account) {
	for _, account := range u.Accounts {
		account := account // Since we're dealing with pointers here, it's best to re-scope the var.
		if account.Type == accountType {
			// KC: This is a hack to appease the gods of poor data quality.
			// At the time of this commit we have many users with duplicate accounts by type.
			// This logic naively assumes the account with the larger id is the preferred account.
			if found == nil || found.ID.Int64() < account.ID.Int64() {
				found = &account
			}
		}
	}

	return
}

func AccountTypeFromSourceType(typeKey string) (string, error) {
	switch typeKey {
	case FacebookTypeKey:
		return FacebookAccountType, nil
	case InstagramTypeKey:
		return InstagramAccountType, nil
	case TwitterTypeKey:
		return TwitterAccountType, nil
	case FoursquareTypeKey:
		return FoursquareAccountType, nil
	case DropboxTypeKey:
		return DropboxAccountType, nil
	case GoogleTypeKey:
		return GoogleAccountType, nil
	default:
		if lang.In(
			typeKey,
			FacebookAccountType,
			InstagramAccountType,
			TwitterAccountType,
			FoursquareAccountType,
			DropboxAccountType,
			GoogleAccountType,
		) {
			return typeKey, nil
		}
		return "", errors.New("Unknown account type key: " + typeKey)
	}
}

func (u User) FacebookAccount() *Account {
	return u.AccountByType(FacebookAccountType)
}

func (u User) TwitterAccount() *Account {
	return u.AccountByType(TwitterAccountType)
}

func (u User) InstagramAccount() *Account {
	return u.AccountByType(InstagramAccountType)
}

func (u User) GoogleAccount() *Account {
	return u.AccountByType(GoogleAccountType)
}

func (u User) DropboxAccount() *Account {
	return u.AccountByType(DropboxAccountType)
}

func (u User) FoursquareAccount() *Account {
	return u.AccountByType(FoursquareAccountType)
}

// Location is guaranteed not to be nil, but might default to America/New_York
func (u User) Location() *time.Location {
	if u.Timezone == "" {
		// If timezone is '' then std lib will load UTC...
		return timehop.EasternTimezone
	}
	loc, err := timehop.Zone(u.Timezone).LoadLocation()
	if err != nil {
		// Fallback to EST if there's a problem.
		log.Warn("User", "Could not load location for user; falling back to EST.", "user_id", u.ID.Int64(), "timezone", u.Timezone)
		return timehop.EasternTimezone
	}
	return loc
}

func (u User) IsAmerican() bool {
	return timehop.Zone(u.Timezone).IsAmerican()
}

func (u User) IsBritish() bool {
	return timehop.Zone(u.Timezone).IsBritish()
}

// HasDesktopPhotos returns true if the user has downloaded either the osx or windows app
func (u User) HasDesktopPhotos() bool {
	return u.DownloadedOSXAppAt != nil || u.DownloadedWindowsAppAt != nil
}

func (u User) HasMobilePhotos() bool {
	if u.Preferences == nil {
		return false
	}
	return u.Preferences.MobilePhotosState.Int() == mobilePhotosRemote
}

// KC: Refactored slightly from ye Date of old
// KC: I'm not 100% sure if this indirection is still necessary. It would
// probably be perfectly suitable to just use a time.Time object or string alias.
// The biggest worry is if someone tries to JSON marshal the non-pointer type.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func NewDate(date string) (*Date, error) {
	t, err := time.Parse("20060102", date)
	if err != nil {
		return nil, err
	}
	if t.Year() == 1 {
		return nil, errors.New("whois: invalid date")
	}
	return &Date{
		Year:  t.Year(),
		Month: t.Month(),
		Day:   t.Day(),
	}, nil
}

// Guarantees that the pointer type will correctly deserialize from a string date value
func (date *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	var t time.Time
	var err error
	if len(s) == 8 { // Indicates 4 digit year
		t, err = time.Parse("20060102", s)
		if err != nil {
			return err
		}
	} else { // Sometimes we get a 2 digit year
		t, err = time.Parse("060102", s)
		if err != nil {
			return err
		}
	}
	date.Year = t.Year()
	date.Month = t.Month()
	date.Day = t.Day()
	return nil
}

// Guarantees that the pointer type will correctly serialize to a string value
func (date *Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(date.String())
}

func (date Date) String() string {
	return fmt.Sprintf("%.4d%.2d%.2d", date.Year, date.Month, date.Day)
}

type AppAuthToken struct {
	ID        thjson.Number `json:"id"`
	UserID    thjson.Number `json:"user_id"`
	AuthToken string        `json:"auth_token"`
	CreatedAt *time.Time    `json:"created_at"`
	ExpiresAt *time.Time    `json:"expires_at"`
}

func (t *AppAuthToken) Equals(t2 *AppAuthToken) bool {
	if reflect.DeepEqual(t, t2) {
		return true
	} else if t == nil || t2 == nil {
		return false
	}

	if t.ID != t2.ID ||
		t.UserID != t2.UserID ||
		t.AuthToken != t2.AuthToken {
		return false
	}

	var createdAt1, createdAt2, expiresAt1, expiresAt2 time.Time

	if t.CreatedAt != nil {
		createdAt1 = (*t.CreatedAt).UTC()
	}
	if t2.CreatedAt != nil {
		createdAt2 = (*t2.CreatedAt).UTC()
	}
	if t.ExpiresAt != nil {
		expiresAt1 = (*t.ExpiresAt).UTC()
	}
	if t2.ExpiresAt != nil {
		expiresAt2 = (*t2.ExpiresAt).UTC()
	}

	if !createdAt1.Equal(createdAt2) || !expiresAt1.Equal(expiresAt2) {
		return false
	}

	return true
}

// The pointer type for WhoisError is never used. If you wish to return
// an error that may be nil, use the error interface and return a nil type.
type WhoisError struct {
	Desc       string        `json:"error"`
	StatusCode thjson.Number `json:"code"`
	Transient  bool          `json:"transient"`
}

// Satisfies the TimehopError interface
func (e WhoisError) IsTransient() bool { return e.Transient }
func (e WhoisError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprint("whois: ", e.StatusCode.Int(), " ", e.Desc)
	}
	return fmt.Sprint("whois: ", e.Desc)
}

func NewError(description string, statusCode int, isTransient bool) WhoisError {
	return WhoisError{
		Desc:       description,
		StatusCode: thjson.Number(statusCode),
		Transient:  isTransient,
	}
}

const (
	mobilePhotosDisabled = 0
	mobilePhotosLocal    = 1
	mobilePhotosRemote   = 2
)

// TODO KEvin S: I think we can now clean up the legacy fields as caching is gone!
type UserPreferences struct {
	// KC: user_id is a new field wrt to cached values. We should be making sure this
	// gets manually set before being returned from the client.
	UserID             thjson.Number `json:"user_id"`
	TweetReplies       bool          `json:"tweet_replies"`
	DailyPush          bool          `json:"daily_push"`
	PushReminders      bool          `json:"push_reminders"`
	EmailReminders     bool          `json:"email_reminders"`
	EmailAnnouncements bool          `json:"email_announcements"`
	MobilePhotosState  thjson.Number `json:"mobile_photos_state"`
	MobileVideosState  thjson.Number `json:"mobile_videos_state"`
	CameraRoll         bool          `json:"camera_roll"`
	/*
	   email: This field's name doesn't match the JSON exported field as the initial
	   version of this field was called just 'email'. Later requirements dictated
	   that it be renamed to 'email_all' but since whois caches take ~a week to
	   recycle, we opted to keep the old JSON field name (which only ever lives
	   in the caches) so as to make deployment faster (i.e. not having to wait
	   another week before rolling anything that depended on the accurate
	   value of this field.)
	*/
	EmailAll bool `json:"email"`
}

type TwitterInfo struct {
	ID              thjson.Number `json:"id"`
	IDStr           string        `json:"id_str"`
	AccountID       thjson.Number `json:"account_id"`
	ScreenName      string        `json:"screen_name"`
	StatusesCount   thjson.Number `json:"statuses_count"`
	Name            string        `json:"name"`
	ProfileImageURL string        `json:"profile_image_url"`
	TimeZone        string        `json:"time_zone"`
	Verified        bool          `json:"verified"`
	UpdatedAt       *time.Time    `json:"updated_at"`
	CreatedAt       *time.Time    `json:"created_at"`
}
