package models

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

type ServiceType struct {
	Type     string
	Services []Service
}

type Service struct {
	Name        string
	ID          string
	Location    string
	Date        string
	Departments []Department
}

type Department struct {
	Name      string
	Positions []Position
}

type Position struct {
	Name       string
	Volunteers []string
}

func RenderServices(bs []byte) ([]ServiceType, error) {
	if !gjson.ValidBytes(bs) {
		return nil, errors.New("invalid json")
	}
	svcList := gjson.GetBytes(bs, "services.service")
	if !svcList.IsArray() {
		return nil, fmt.Errorf("services.service: expected array, got %v", svcList.Type)
	}

	serviceTypes := make(map[string][]Service)
	for _, svc := range svcList.Array() {
		st := svc.Get("service_type.name").String()
		serviceTypes[st] = append(serviceTypes[st], Service{
			Name:        svc.Get("name").String(),
			ID:          svc.Get("id").String(),
			Location:    svc.Get("location.name").String(),
			Date:        strings.Split(svc.Get("date").String(), " ")[0],
			Departments: getDepartments(svc.Get("volunteers")),
		})
	}

	var res []ServiceType
	for st, sx := range serviceTypes {
		res = append(res, ServiceType{Type: st, Services: sx})
	}
	return res, nil
}

func getDepartments(json gjson.Result) []Department {
	depts := make(map[string][]Position)
	for _, plan := range json.Get("plan").Array() {
		for _, pos := range plan.Get("positions.position").Array() {
			if pos.Get("volunteers").String() == "" {
				continue
			}
			dept := pos.Get("department_name").String()
			depts[dept] = append(depts[dept], Position{
				Name:       pos.Get("position_name").String(),
				Volunteers: volunteerNames(pos.Get("volunteers.volunteer")),
			})
		}
	}
	var res []Department
	for d, px := range depts {
		res = append(res, Department{
			Name:      d,
			Positions: px,
		})
	}
	return res
}

func volunteerNames(json gjson.Result) []string {
	var res []string
	json.ForEach(func(_, val gjson.Result) bool {
		res = append(res, fmt.Sprintf("%s %s", val.Get("person.firstname").String(), val.Get("person.lastname").String()))
		return true
	})
	return res
}
