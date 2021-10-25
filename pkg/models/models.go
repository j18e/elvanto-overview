package models

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type TokenPair struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func RenderServices(r io.Reader) ([]ServiceType, error) {
	var data ServicesResponse
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}
	services := make(map[string][]Service)
	for _, svc := range data.Services.Service {
		service := Service{
			Name:       svc.Name,
			ID:         svc.ID,
			Location:   svc.Location.Name,
			Date:       svc.Date,
			Volunteers: svc.Volunteers,
		}
		services[svc.Type.Name] = append(services[svc.Type.Name], service)
	}
	var serviceTypes []ServiceType
	for t, sx := range services {
		serviceTypes = append(serviceTypes, ServiceType{
			Type:     t,
			Services: sx,
		})
	}

	sort.Slice(serviceTypes, func(i, j int) bool {
		return serviceTypes[i].Type < serviceTypes[j].Type
	})

	return serviceTypes, nil
}

type ServiceType struct {
	Type     string
	Services []Service
}

type Service struct {
	Name       string
	ID         string
	Location   string
	Date       string
	Volunteers []Volunteer
}

func (s Service) String() string {
	res := fmt.Sprintf("%s: %s at %s:", s.Date, s.Name, s.Location)
	for _, v := range s.Volunteers {
		res += "\n\t" + v.String()
	}
	return res
}

type Volunteer struct {
	Name       string
	Department string
	Position   string
}

func (v Volunteer) String() string {
	return fmt.Sprintf("%s/%s: %s", v.Department, v.Position, v.Name)
}

type ServicesResponse struct {
	GeneratedIn string `json:"generated_in"`
	Services    struct {
		// OnThisPage int    `json:"on_this_page"`
		// Page       string `json:"page"`
		// PerPage    string `json:"per_page"`
		Service []struct {
			ID           string `json:"id"`
			Date         string `json:"date"`
			DateAdded    string `json:"date_added"`
			DateModified string `json:"date_modified"`
			Description  string `json:"description"`
			Location     struct {
				Name string `json:"name"`
			} `json:"location"`
			Name string `json:"name"`
			Type struct {
				Name string `json:"name"`
			} `json:"service_type"`
			Status     int           `json:"status"`
			Volunteers VolunteerList `json:"volunteers"`
		} `json:"service"`
		Total int `json:"total"`
	} `json:"services"`
	Status string `json:"status"`
}

type VolunteerList []Volunteer

func (vl *VolunteerList) UnmarshalJSON(bs []byte) error {
	var data struct {
		Plan []struct {
			Positions struct {
				Position []struct {
					Department    string             `json:"department_name"`
					Position      string             `json:"position_name"`
					SubDepartment string             `json:"sub_department_name"`
					Volunteers    ResponseVolunteers `json:"volunteers"`
				} `json:"position"`
			} `json:"positions"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(bs, &data); err != nil {
		return err
	}
	var res VolunteerList
	for _, plan := range data.Plan {
		for _, pos := range plan.Positions.Position {
			for _, vol := range pos.Volunteers {
				res = append(res, Volunteer{
					Name:       vol,
					Department: pos.Department,
					Position:   pos.Position,
				})
			}
		}
	}
	*vl = res
	return nil
}

type ResponseVolunteers []string

func (v *ResponseVolunteers) UnmarshalJSON(bs []byte) error {
	if string(bs) == `""` {
		return nil
	}
	var data struct {
		Volunteer []struct {
			Person struct {
				Firstname     string `json:"firstname"`
				Lastname      string `json:"lastname"`
				MiddleName    string `json:"middle_name"`
				PreferredName string `json:"preferred_name"`
			} `json:"person"`
			Status string `json:"status"`
		} `json:"volunteer"`
	}
	if err := json.Unmarshal(bs, &data); err != nil {
		return err
	}
	var res []string
	for _, pers := range data.Volunteer {
		res = append(res, fmt.Sprintf("%s %s", pers.Person.Firstname, pers.Person.Lastname))
	}
	*v = res
	return nil
}
