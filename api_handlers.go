package main

import "net/http"
import "encoding/json"
import "strconv"
import "strings"
import "fmt"


type Response map[string]interface{}

func writeJsonResponse(w http.ResponseWriter, response interface{}, status int) {
  jsonResponse, _ := json.Marshal(response)
  w.WriteHeader(status)
  _, err := w.Write(jsonResponse)
  if err != nil {
    panic("Unexpected error")
  }
}

type IntMaxValidator struct {
  rule string
  paramName string
  Max int
}

type IntMinValidator struct {
  rule string
  paramName string
  Min int
}

type StringMaxValidator struct {
  rule string
  paramName string
  Max int
}

type StringMinValidator struct {
  rule string
  paramName string
  Min int
}

type EnumValidator struct {
  rule string
  paramName string
  list map[string]int
}

type RequiredValidator struct {
  rule string
  paramName string
}

func (v IntMaxValidator) Validate(val interface{}) (bool, error) {
  num := val.(int)

  if num > v.Max {
    return false, fmt.Errorf(v.paramName + " must be <= %v", v.Max)
  }
  
  return true, nil
}

func (v IntMinValidator) Validate(val interface{}) (bool, error) {
  num := val.(int)

  if num < v.Min {
    return false, fmt.Errorf(v.paramName + " must be >= %v", v.Min)
  }
  
  return true, nil
}

func (v StringMaxValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if v.Max > 0 && l > v.Max {
    return false, fmt.Errorf(v.paramName + " must be <= %v", v.Max)
  }
  
  return true, nil
}

func (v StringMinValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if v.Min > 0 && l < v.Min {
    return false, fmt.Errorf(v.paramName + " len must be >= %v", v.Min)
  }
  
  return true, nil
}

func (v EnumValidator) Validate(val interface{}) (bool, error) {
  item := val.(string)

  if _,ok := v.list[item]; !ok {
	return false, fmt.Errorf(v.paramName + " must be one of [" + strings.Join(strings.Split(strings.Split(v.rule, "=")[1], "|"), ", ") + "]")
  }
  
  return true, nil
}

func (v RequiredValidator) Validate(val interface{}) (bool, error) {
  l := len(val.(string))

  if l == 0 {
	return false, fmt.Errorf(v.paramName + " must me not empty")
  }
  
  return true, nil
}

func (h *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {

    case "/user/create":
		
		token := r.Header.Get("X-Auth")
		if token != "100500" {
			writeJsonResponse(w, Response{"error": "unauthorized"}, http.StatusForbidden)
			return
		}
		
		
		if r.Method != "POST" {
			writeJsonResponse(w, Response{"error": "bad method"}, http.StatusNotAcceptable)
			return
		}
		
        h.handlerCreate(w, r)

    default:
        // 404
		writeJsonResponse(w, Response{"error": "unknown method"}, http.StatusNotFound)
    }
}

func (h *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := http.StatusOK
    errorMessage := ""
	
    // валидирование параметров
	// заполнение структуры params
	params := OtherCreateParams{}
    paramName := ""

	

    paramName = strings.ToLower("Username")
    

    
    params.Username = r.FormValue(paramName)
    
    
	
    if ok, err := (RequiredValidator{"required", paramName}).Validate(params.Username); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    if ok, err := (StringMinValidator{"min=3", paramName, 3 }).Validate(params.Username); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

    paramName = strings.ToLower("Name")
    paramName = "account_name"

    
    params.Name = r.FormValue(paramName)
    
    
	
    

    paramName = strings.ToLower("Class")
    

    
    params.Class = r.FormValue(paramName)
    
    
	
	if params.Class == "" {
	 params.Class = "warrior"
	}
    if ok, err := (EnumValidator{"enum=warrior|sorcerer|rouge", paramName, map[string]int{ "warrior":1, "sorcerer":1, "rouge": 1 }}).Validate(params.Class); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

    paramName = strings.ToLower("Level")
    

    
    
    value, err := strconv.Atoi(r.FormValue(paramName))
    if err != nil {
      writeJsonResponse(w, Response{"error": paramName + " must be int"}, http.StatusBadRequest)
      return
	}
	params.Level = value
    
	
    if ok, err := (IntMinValidator{"min=1", paramName, 1 }).Validate(params.Level); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    if ok, err := (IntMaxValidator{"max=50", paramName, 50 }).Validate(params.Level); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

	result, err := h.Create(ctx, params)

	// прочие обработки
	if err != nil {
		switch err.(type) {
		case ApiError:
			err := err.(ApiError)
			status = err.HTTPStatus
			errorMessage = err.Error()
		default:
            status = http.StatusInternalServerError
			errorMessage = err.Error()
		} 

		writeJsonResponse(w, Response{
			"error": errorMessage,
    	}, status)
		return 
	}

	writeJsonResponse(w, Response{
        "error": "",
		"response": result,
    }, status)
}

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {

    case "/user/profile":
		
		
        h.handlerProfile(w, r)

    case "/user/create":
		
		token := r.Header.Get("X-Auth")
		if token != "100500" {
			writeJsonResponse(w, Response{"error": "unauthorized"}, http.StatusForbidden)
			return
		}
		
		
		if r.Method != "POST" {
			writeJsonResponse(w, Response{"error": "bad method"}, http.StatusNotAcceptable)
			return
		}
		
        h.handlerCreate(w, r)

    default:
        // 404
		writeJsonResponse(w, Response{"error": "unknown method"}, http.StatusNotFound)
    }
}

func (h *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := http.StatusOK
    errorMessage := ""
	
    // валидирование параметров
	// заполнение структуры params
	params := ProfileParams{}
    paramName := ""

	

    paramName = strings.ToLower("Login")
    

    
    params.Login = r.FormValue(paramName)
    
    
	
    if ok, err := (RequiredValidator{"required", paramName}).Validate(params.Login); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

	result, err := h.Profile(ctx, params)

	// прочие обработки
	if err != nil {
		switch err.(type) {
		case ApiError:
			err := err.(ApiError)
			status = err.HTTPStatus
			errorMessage = err.Error()
		default:
            status = http.StatusInternalServerError
			errorMessage = err.Error()
		} 

		writeJsonResponse(w, Response{
			"error": errorMessage,
    	}, status)
		return 
	}

	writeJsonResponse(w, Response{
        "error": "",
		"response": result,
    }, status)
}

func (h *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := http.StatusOK
    errorMessage := ""
	
    // валидирование параметров
	// заполнение структуры params
	params := CreateParams{}
    paramName := ""

	

    paramName = strings.ToLower("Login")
    

    
    params.Login = r.FormValue(paramName)
    
    
	
    if ok, err := (RequiredValidator{"required", paramName}).Validate(params.Login); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    if ok, err := (StringMinValidator{"min=10", paramName, 10 }).Validate(params.Login); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

    paramName = strings.ToLower("Name")
    paramName = "full_name"

    
    params.Name = r.FormValue(paramName)
    
    
	
    

    paramName = strings.ToLower("Status")
    

    
    params.Status = r.FormValue(paramName)
    
    
	
	if params.Status == "" {
	 params.Status = "user"
	}
    if ok, err := (EnumValidator{"enum=user|moderator|admin", paramName, map[string]int{ "user":1, "moderator":1, "admin": 1 }}).Validate(params.Status); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

    paramName = strings.ToLower("Age")
    

    
    
    value, err := strconv.Atoi(r.FormValue(paramName))
    if err != nil {
      writeJsonResponse(w, Response{"error": paramName + " must be int"}, http.StatusBadRequest)
      return
	}
	params.Age = value
    
	
    if ok, err := (IntMinValidator{"min=0", paramName, 0 }).Validate(params.Age); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    if ok, err := (IntMaxValidator{"max=128", paramName, 128 }).Validate(params.Age); !ok && err != nil {
      writeJsonResponse(w, Response{
        "error": err.Error(),
      }, http.StatusBadRequest)
      return
    }

    

	result, err := h.Create(ctx, params)

	// прочие обработки
	if err != nil {
		switch err.(type) {
		case ApiError:
			err := err.(ApiError)
			status = err.HTTPStatus
			errorMessage = err.Error()
		default:
            status = http.StatusInternalServerError
			errorMessage = err.Error()
		} 

		writeJsonResponse(w, Response{
			"error": errorMessage,
    	}, status)
		return 
	}

	writeJsonResponse(w, Response{
        "error": "",
		"response": result,
    }, status)
}
