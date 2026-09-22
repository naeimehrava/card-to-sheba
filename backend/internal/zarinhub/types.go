package zarinhub

type envelope[T any] struct {
	Data T    `json:"data"`
	Meta meta `json:"meta"`
}

type meta struct {
	Code         int    `json:"code"`
	ErrorMessage string `json:"errorMessage"`
	ErrorType    string `json:"errorType"`
	Errors       []any  `json:"errors"`
	IsSuccess    bool   `json:"isSuccess"`
	Message      string `json:"message"`
	Status       string `json:"status"`
	TrackID      string `json:"trackId"`
}

const (
	codeSuccess       = 0
	codeServerError   = 1
	codeBadRequest    = 2
	codeNotFound      = 3
	codeListEmpty     = 4
	codeLogicError    = 5
	codeUnauthorized  = 6
	codeUnavailable   = 7
	codeInaccessible  = 9
	codeRequestTimout = 14
	codeProviderError = 15
)

type cardToIBANRequest struct {
	Card string `json:"card"`
}

type cardToIBANData struct {
	BankName           string `json:"bankName"`
	Card               string `json:"card"`
	Deposit            string `json:"deposit"`
	DepositDescription string `json:"depositDescription"`
	DepositOwners      string `json:"depositOwners"`
	DepositStatus      string `json:"depositStatus"`
	EnglishBankName    string `json:"englishBankName"`
	IBAN               string `json:"iban"`
}

type getTokenRequest struct {
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

type getTokenData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresTn    int64  `json:"expiresTn"`
}
