package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ServerCep struct {
	RequestNameOTEL string
	OTELTracer      trace.Tracer
}

type ViaCEP struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

type CepInput struct {
	CEP string `json:"cep"`
}

type CepTemperatures struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

func NewServerCep(requestNameOTEL string, otelTracer trace.Tracer) *ServerCep {
	return &ServerCep{
		RequestNameOTEL: requestNameOTEL,
		OTELTracer:      otelTracer,
	}
}

func (we *ServerCep) CreateServer() *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))
	// promhttp
	router.Handle("/metrics", promhttp.Handler())
	router.Post("/", we.ValidateCep)
	return router
}

func (h *ServerCep) ValidateCep(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, span := h.OTELTracer.Start(ctx, h.RequestNameOTEL)
	defer span.End()

	var cepData CepInput
	err := json.NewDecoder(r.Body).Decode(&cepData)
	if err != nil {
		http.Error(w, "CEP is required", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	valid, err := VerifyValidCEP(cepData.CEP)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !valid {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	cep, err := GetCEPInfo(cepData.CEP)
	if err != nil {
		if err.Error() == "can not find zipcode" {
			http.Error(w, err.Error(), http.StatusNotFound)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		http.Error(w, "Error getting CEP info: "+err.Error(), http.StatusBadRequest)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://weather:8181?cep="+url.QueryEscape(cep.Cep), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "Error getting weather info: "+err.Error(), http.StatusBadRequest)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var temperatures CepTemperatures

	err = json.Unmarshal(body, &temperatures)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(temperatures)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func VerifyValidCEP(cep string) (bool, error) {
	valid, err := regexp.MatchString("^\\d{5}-*\\d{3}$", cep)
	if err != nil {
		return false, err
	}

	return valid, nil
}

func GetCEPInfo(cep string) (*ViaCEP, error) {
	resp, err := http.Get("http://viacep.com.br/ws/" + cep + "/json/")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "erro") {
		return nil, errors.New("can not find zipcode")
	}

	var cepData ViaCEP

	err = json.Unmarshal(body, &cepData)
	if err != nil {
		return nil, err
	}

	return &cepData, nil
}
