package ksml

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

func isType(t string, vals ...reflect.Value) bool {
	for _, val := range vals {
		if !strings.Contains(val.Type().String(), t) {
			return false
		}
	}
	return true
}

func parseInt(val interface{}) (int64, error) {
	v, err := InterfaceToString(val)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func Mul(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, fmt.Errorf("invalid arguments, expect 2 got %v", len(extras))
	}
	vals, err := p.handleContents(extras)
	if err != nil {
		return nil, err
	}

	// convert to big.Int or big.Float if returned vals are float64 or int64
	for i, _ := range vals {
		if reflect.ValueOf(vals[i]).Kind() == reflect.Float64 {
			vals[i] = big.NewFloat(reflect.ValueOf(vals[i]).Float())
		} else if reflect.ValueOf(vals[i]).Kind() == reflect.Int64 {
			vals[i] = big.NewInt(reflect.ValueOf(vals[i]).Int())
		}
	}

	val1, val2 := reflect.ValueOf(vals[0]), reflect.ValueOf(vals[1])
	if isType("big.Int", val1, val2) {
		return []interface{}{big.NewInt(0).Mul(val1.Interface().(*big.Int), val2.Interface().(*big.Int))}, nil
	} else if isType("big.Float", val1, val2) {
		return []interface{}{big.NewFloat(0).Mul(val1.Interface().(*big.Float), val2.Interface().(*big.Float))}, nil
	}
	return nil, fmt.Errorf("unsupport type %v or %v in Mul func, expect big.Int or big.Float", val1.Type().String(), val2.Type().String())
}

func Div(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, fmt.Errorf("invalid arguments, expect 2 got %v", len(extras))
	}
	vals, err := p.handleContents(extras)
	if err != nil {
		return nil, err
	}

	// convert to big.Int or big.Float if returned vals are float64 or int64
	for i, _ := range vals {
		if reflect.ValueOf(vals[i]).Kind() == reflect.Float64 {
			vals[i] = big.NewFloat(reflect.ValueOf(vals[i]).Float())
		} else if reflect.ValueOf(vals[i]).Kind() == reflect.Int64 {
			vals[i] = big.NewInt(reflect.ValueOf(vals[i]).Int())
		}
	}

	val1, val2 := reflect.ValueOf(vals[0]), reflect.ValueOf(vals[1])
	if isType("big.Int", val1, val2) {
		return []interface{}{big.NewInt(0).Div(val1.Interface().(*big.Int), val2.Interface().(*big.Int))}, nil
	} else if isType("big.Float", val1, val2) {
		return []interface{}{big.NewFloat(0).Quo(val1.Interface().(*big.Float), val2.Interface().(*big.Float))}, nil
	}
	return nil, fmt.Errorf("unsupport type %v or %v in Div func, expect big.Int or big.Float", val1.Type().String(), val2.Type().String())
}

func SetInt(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 1 {
		return nil, fmt.Errorf("invalid arguments, expect 1 got %v", len(extras))
	}

	// handle content
	c, err := p.handleContent(extras[0].(string))
	if err != nil {
		return nil, err
	}

	// convert returned result to string
	str, err := InterfaceToString(c[0])
	if err != nil {
		return nil, err
	}

	// if str contains '.' convert to float before convert to int
	if strings.Contains(str, ".") {
		f, ok := big.NewFloat(0).SetString(str)
		if !ok {
			return nil, fmt.Errorf("cannot convert %v to big.Float", str)
		}
		int64Value, _ := f.Int64()
		return []interface{}{big.NewInt(int64Value)}, nil
	}

	v, r := big.NewInt(0).SetString(str, 10)
	if !r {
		return nil, fmt.Errorf("cannot convert %v to big.Int", extras[0])
	}
	return []interface{}{v}, nil
}

func setFloat(p *Parser, extras ...interface{}) (*big.Float, error) {
	if len(extras) != 1 {
		return nil, fmt.Errorf("invalid arguments, expect 1 got %v", len(extras))
	}
	// handle content
	c, err := p.handleContent(extras[0].(string))
	if err != nil {
		return nil, err
	}
	// convert returned result to string
	str, err := InterfaceToString(c[0])
	if err != nil {
		return nil, err
	}

	v, r := big.NewFloat(0).SetString(str)
	if !r {
		return nil, fmt.Errorf("cannot convert %v to big.Float", reflect.ValueOf(c[0]).Type().String())
	}
	return v, nil
}

func SetFloat(p *Parser, extras ...interface{}) ([]interface{}, error) {
	v, err := setFloat(p, extras...)
	if err != nil {
		return nil, err
	}
	return []interface{}{v}, nil
}

func Round(p *Parser, extras ...interface{}) ([]interface{}, error) {
	v, err := setFloat(p, extras...)
	if err != nil {
		return nil, err
	}
	val, _ := v.Float64()
	return []interface{}{math.Round(val)}, nil
}

func Exp(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, fmt.Errorf("invalid arguments, expect 2 got %v", len(extras))
	}
	vals, err := p.handleContents(extras)
	if err != nil {
		return nil, err
	}
	val1, val2 := reflect.ValueOf(vals[0]), reflect.ValueOf(vals[1])
	if isType("big.Int", val1, val2) {
		return []interface{}{big.NewInt(0).Exp(val1.Interface().(*big.Int), val2.Interface().(*big.Int), nil)}, nil
	} else if isType("int", val1, val2) {
		v1, v2 := big.NewInt(val1.Int()), big.NewInt(val2.Int())
		return []interface{}{big.NewInt(0).Exp(v1, v2, nil)}, nil
	}
	return nil, fmt.Errorf("unsupport type %v or %v in Exp func, expect big.Int", val1.Type().String(), val2.Type().String())
}

func FormatFloat(p *Parser, extras ...interface{}) ([]interface{}, error) {
	if len(extras) != 2 {
		return nil, fmt.Errorf("invalid arguments, expect 2 got %v", len(extras))
	}
	vals, err := p.handleContents(extras)
	if err != nil {
		return nil, err
	}

	precision, err := parseInt(vals[1])
	if err != nil {
		return nil, err
	}

	format := "%." + strconv.Itoa(int(precision)) + "f"
	val1 := reflect.ValueOf(vals[0])
	if isType("big.Float", val1) {
		floatVal := vals[0].(*big.Float)
		v, err := strconv.ParseFloat(fmt.Sprintf(format, floatVal.String()), 64)
		if err != nil {
			return nil, err
		}
		return []interface{}{v}, nil
	}
	return nil, fmt.Errorf("unsupport type %v in format func, expect big.Float", val1.Type().String())
}
