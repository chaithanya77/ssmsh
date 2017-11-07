package parameterstore_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/kountable/pssh/parameterstore"
)

var EddardStark = &ssm.Parameter{
	Name:  aws.String("/House/Stark/EddardStark"),
	Type:  aws.String("String"),
	Value: aws.String("Lord"),
}

var CatelynStark = &ssm.Parameter{
	Name:  aws.String("/House/Stark/CatelynStark"),
	Type:  aws.String("String"),
	Value: aws.String("Lady"),
}

var RobStark = &ssm.Parameter{
	Name:  aws.String("/House/Stark/RobStark"),
	Type:  aws.String("String"),
	Value: aws.String("Noble"),
}

var JonSnow = &ssm.Parameter{
	Name:  aws.String("/House/Stark/JonSnow"),
	Type:  aws.String("String"),
	Value: aws.String("Bastard"),
}

var DaenerysTargaryen = &ssm.Parameter{
	Name:  aws.String("/House/Targaryen/DaenerysTargaryen"),
	Type:  aws.String("String"),
	Value: aws.String("Noble"),
}

var HouseStark = []*ssm.Parameter{
	EddardStark,
	CatelynStark,
	RobStark,
}

var HouseStarkNext = []*ssm.Parameter{
	JonSnow,
}

var HouseTargaryen = []*ssm.Parameter{
	DaenerysTargaryen,
}

const NextToken = "A1B2C3D4"

type mockedSSM struct {
	ssmiface.SSMAPI
	GetParametersByPathResp ssm.GetParametersByPathOutput
	GetParametersByPathNext ssm.GetParametersByPathOutput
	GetParameterHistoryResp ssm.GetParameterHistoryOutput
	GetParametersResp       ssm.GetParametersOutput
	GetParameterResp        []ssm.GetParameterOutput
	DeleteParametersResp    ssm.DeleteParametersOutput
	PutParameterResp        ssm.PutParameterOutput
}

func (m mockedSSM) GetParametersByPath(in *ssm.GetParametersByPathInput) (*ssm.GetParametersByPathOutput, error) {
	if aws.StringValue(in.NextToken) != "" {
		return &m.GetParametersByPathNext, nil
	}
	return &m.GetParametersByPathResp, nil
}

func (m mockedSSM) DeleteParameters(in *ssm.DeleteParametersInput) (*ssm.DeleteParametersOutput, error) {
	return &m.DeleteParametersResp, nil
}

func (m mockedSSM) GetParameter(in *ssm.GetParameterInput) (*ssm.GetParameterOutput, error) {
	parameterName := aws.StringValue(in.Name)
	for _, param := range m.GetParameterResp {
		if aws.StringValue(param.Parameter.Name) == parameterName {
			return &param, nil
		}
	}
	return nil, errors.New("Parameter not found")
}

func (m mockedSSM) GetParameterHistory(in *ssm.GetParameterHistoryInput) (*ssm.GetParameterHistoryOutput, error) {
	return &m.GetParameterHistoryResp, nil
}

func (m mockedSSM) GetParameters(in *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
	for _, n := range in.Names {
		input := &ssm.GetParameterInput{
			Name:           n,
			WithDecryption: aws.Bool(true),
		}
		parameter, err := m.GetParameter(input)
		if err != nil {
			m.GetParametersResp.InvalidParameters = append(m.GetParametersResp.InvalidParameters, n)
		} else {
			m.GetParametersResp.Parameters = append(m.GetParametersResp.Parameters, parameter.Parameter)
		}
	}
	return &m.GetParametersResp, nil
}

func (m mockedSSM) PutParameter(in *ssm.PutParameterInput) (*ssm.PutParameterOutput, error) {
	return &m.PutParameterResp, nil
}

func TestPut(t *testing.T) {
	var expectedVersion int64 = 1
	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Cwd = parameterstore.Delimiter
	p.Client = mockedSSM{
		PutParameterResp: ssm.PutParameterOutput{
			Version: aws.Int64(expectedVersion),
		},
	}
	putParameterInput := ssm.PutParameterInput{
		Name:        aws.String("/House/Stark/EddardStark"),
		Value:       aws.String("Lord"),
		Description: aws.String("Lord of Winterfell in Season 1"),
		Type:        aws.String("String"),
	}
	resp, err := p.Put(&putParameterInput)
	if err != nil {
		t.Fatal("Error putting parameter", err)
	} else {
		if aws.Int64Value(resp.Version) != expectedVersion {
			msg := fmt.Errorf("expected %d, got %d", expectedVersion, aws.Int64Value(resp.Version))
			t.Fatal(msg)
		}
	}
}

func TestMoveParameter(t *testing.T) {
	srcParam := "/House/Stark/SansaStark"
	dstParam := "/House/Lannister/SansaStark"
	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Cwd = parameterstore.Delimiter
	p.Client = mockedSSM{
		GetParameterResp: []ssm.GetParameterOutput{
			{
				Parameter: &ssm.Parameter{
					Name:  aws.String(srcParam),
					Type:  aws.String("String"),
					Value: aws.String("Noble"),
				},
			},
			{
				Parameter: &ssm.Parameter{
					Name:  aws.String(dstParam),
					Type:  aws.String("String"),
					Value: aws.String("Noble"),
				},
			},
		},
		GetParameterHistoryResp: ssm.GetParameterHistoryOutput{
			Parameters: []*ssm.ParameterHistory{
				{
					Name:        aws.String(srcParam),
					Value:       aws.String("Noble"),
					Type:        aws.String("String"),
					Description: aws.String("Eldest daughter of House Stark, bethrothed to Tyrion Lannister"),
					Version:     aws.Int64(2),
				},
				{
					Name:        aws.String(srcParam),
					Value:       aws.String("Noble"),
					Type:        aws.String("String"),
					Description: aws.String("Eldest daughter of House Stark"),
					Version:     aws.Int64(1),
				},
			},
		},
	}
	err := p.Move(srcParam, dstParam)
	if err != nil {
		t.Fatal("Error moving parameter", err)
	}
	p.Client = mockedSSM{
		GetParameterResp: []ssm.GetParameterOutput{
			{
				Parameter: &ssm.Parameter{
					Name:  aws.String(dstParam),
					Type:  aws.String("String"),
					Value: aws.String("Noble"),
				},
			},
		},
	}
	resp, err := p.Get([]string{srcParam})
	if err != nil {
		msg := fmt.Errorf("Error getting %s: %s", srcParam, err)
		t.Fatal(msg)
	}
	if len(resp) > 0 {
		if err != nil {
			msg := fmt.Errorf("Expected parameter %s to be removed but found %v", srcParam, resp)
			t.Fatal(msg)
		}
	}
	_, err = p.Get([]string{dstParam})
	if err != nil {
		msg := fmt.Errorf("Expected to find %s but didn't!", dstParam)
		t.Fatal(msg)
	}
}

func TestCopyPath(t *testing.T) {
	srcPath := "/House/Stark"
	dstPath := "/House/Targaryen"

	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Cwd = parameterstore.Delimiter
	bothHouses := append(HouseStark, HouseTargaryen...)
	p.Client = mockedSSM{
		GetParameterResp: []ssm.GetParameterOutput{
			{Parameter: EddardStark},
			{Parameter: CatelynStark},
			{Parameter: RobStark},
			{Parameter: JonSnow},
			{Parameter: DaenerysTargaryen},
		},
		GetParametersByPathResp: ssm.GetParametersByPathOutput{
			Parameters: bothHouses,
			NextToken:  aws.String(NextToken),
		},
		GetParametersByPathNext: ssm.GetParametersByPathOutput{
			Parameters: []*ssm.Parameter{JonSnow},
			NextToken:  aws.String(""),
		},
		GetParameterHistoryResp: ssm.GetParameterHistoryOutput{
			Parameters: []*ssm.ParameterHistory{
				{
					Name:    aws.String("/House/Stark/EddardStark"),
					Version: aws.Int64(2),
				},
			},
			NextToken: aws.String(""),
		},
	}
	err := p.Copy(srcPath, dstPath, true)
	if err != nil {
		t.Fatal("Error copying parameter path: ", err)
	}
	expectedName := "/House/Targaryen/Stark/EddardStark"
	resp, err := p.GetHistory(expectedName)
	if err != nil {
		t.Fatal("Error getting history: ", err)
	}
	if len(resp) != 1 {
		msg := fmt.Errorf("Expected history of length 1, got %s", resp)
		t.Fatal(msg)
	}
}

func TestCopyParameter(t *testing.T) {
	srcParam := "/House/Stark/JonSnow"
	dstParam := "/House/Targaryen/JonSnow"
	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Cwd = parameterstore.Delimiter
	p.Client = mockedSSM{
		GetParameterResp: []ssm.GetParameterOutput{
			{
				Parameter: &ssm.Parameter{
					Name:  aws.String("/House/Stark/JonSnow"),
					Type:  aws.String("String"),
					Value: aws.String("King"),
				},
			},
			{
				Parameter: &ssm.Parameter{
					Name:  aws.String("/House/Targaryen/JonSnow"),
					Type:  aws.String("String"),
					Value: aws.String("King"),
				},
			},
		},
		GetParameterHistoryResp: ssm.GetParameterHistoryOutput{
			Parameters: []*ssm.ParameterHistory{
				{
					Name:        aws.String("/House/Stark/JonSnow"),
					Value:       aws.String("King"),
					Type:        aws.String("String"),
					Description: aws.String("King of the north"),
					Version:     aws.Int64(2),
				},
				{
					Name:        aws.String("/House/Stark/JonSnow"),
					Value:       aws.String("Bastard"),
					Type:        aws.String("String"),
					Description: aws.String("Bastard of Winterfell"),
					Version:     aws.Int64(1),
				},
			},
		},
	}
	err := p.Copy(srcParam, dstParam, false)
	if err != nil {
		t.Fatal("Error copying parameter", err)
	}
	resp, err := p.Get([]string{dstParam})
	if err != nil {
		t.Fatal("Error getting parameter", err)
	}
	expectedName := "/House/Targaryen/JonSnow"
	if aws.StringValue(resp[0].Name) != expectedName {
		msg := fmt.Errorf("expected %s, got %s", expectedName, aws.StringValue(resp[0].Name))
		t.Fatal(msg)
	}
}

func TestCwd(t *testing.T) {
	cases := []struct {
		GetParametersByPathResp ssm.GetParametersByPathOutput
		Path                    string
		Expected                string
	}{
		{
			Path:     "/",
			Expected: "/",
		},
		{
			Path: "/House/Stark/..///Deceased",
			GetParametersByPathResp: ssm.GetParametersByPathOutput{
				Parameters: []*ssm.Parameter{
					{
						Name:  aws.String("/House/Stark/EddardStark"),
						Type:  aws.String("String"),
						Value: aws.String("Lord"),
					},
				},
				NextToken: aws.String(""),
			},
			Expected: "/House/Deceased",
		},
	}

	var p parameterstore.ParameterStore
	for _, c := range cases {
		p.NewParameterStore()
		p.Cwd = parameterstore.Delimiter
		p.Client = mockedSSM{
			GetParametersByPathResp: c.GetParametersByPathResp,
		}
		err := p.SetCwd(c.Path)
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		if p.Cwd != c.Expected {
			msg := fmt.Errorf("expected %v, got %v", c.Expected, p.Cwd)
			t.Fatal(msg)
		}
	}

	p.NewParameterStore()
	p.Cwd = parameterstore.Delimiter
	testDir := "/nodir"
	err := p.SetCwd(testDir)
	if err == nil {
		msg := fmt.Errorf("Expected error for dir %s, got cwd %s ", testDir, p.Cwd)
		t.Fatal(msg)
	}
}

func TestDelete(t *testing.T) {
	testParams := []string{
		"/House/Stark/EddardStark",
		"/House/Stark/CatelynStark",
		"/House/Stark/TyrionLannister",
	}
	deleteParametersOutput := ssm.DeleteParametersOutput{
		DeletedParameters: []*string{
			aws.String("/House/Stark/EddardStark"),
			aws.String("/House/Stark/CatelynStark"),
		},
		InvalidParameters: []*string{
			aws.String("/House/Stark/TyrionLannister"),
		},
	}

	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Client = mockedSSM{
		DeleteParametersResp: deleteParametersOutput,
	}
	err := p.Remove(testParams, false)
	if err == nil {
		msg := fmt.Errorf("Expected error for param %s, got %v ", testParams[2], err)
		t.Fatal(msg)
	}
}

func TestGetHistory(t *testing.T) {
	testParam := "/House/Stark/EddardStark"
	getHistoryOutput := ssm.GetParameterHistoryOutput{
		Parameters: []*ssm.ParameterHistory{
			{
				Name:    aws.String("/House/Stark/EddardStark"),
				Version: aws.Int64(2),
			},
			{
				Name:    aws.String("/House/Stark/EddardStark"),
				Version: aws.Int64(1),
			},
		},
		NextToken: aws.String(""),
	}
	var p parameterstore.ParameterStore
	p.NewParameterStore()
	p.Client = mockedSSM{
		GetParameterHistoryResp: getHistoryOutput,
	}
	resp, err := p.GetHistory(testParam)
	if err != nil {
		msg := fmt.Errorf("Unexpected error %s", err)
		t.Fatal(msg)
	}
	if len(resp) != 2 {
		msg := fmt.Errorf("Expected history of length 2, got %s", resp)
		t.Fatal(msg)
	}
}

func TestList(t *testing.T) {
	cases := []struct {
		Query                   string
		GetParametersByPathResp ssm.GetParametersByPathOutput
		GetParametersResp       ssm.GetParametersOutput
		GetParametersByPathNext ssm.GetParametersByPathOutput
		Expected                []string
		Recurse                 bool
	}{
		{
			Query:   "/House/Stark/EddardStark",
			Recurse: false,
			GetParametersByPathResp: ssm.GetParametersByPathOutput{
				Parameters: []*ssm.Parameter{},
				NextToken:  aws.String(""),
			},
			Expected: []string{
				"/House/Stark/EddardStark",
			},
			GetParametersResp: ssm.GetParametersOutput{
				Parameters: []*ssm.Parameter{
					{
						Name:  aws.String("/House/Stark/EddardStark"),
						Type:  aws.String("String"),
						Value: aws.String("Lord"),
					},
				},
			},
		}, {
			Query:   "/",
			Recurse: false,
			Expected: []string{
				"root",
			},
			GetParametersResp: ssm.GetParametersOutput{
				Parameters: []*ssm.Parameter{
					{
						Name:  aws.String("root"),
						Type:  aws.String("String"),
						Value: aws.String("A root parameter"),
					},
				},
			},
		},
		{
			Query:   "/House/Stark",
			Recurse: false,
			GetParametersByPathResp: ssm.GetParametersByPathOutput{
				Parameters: HouseStark,
				NextToken:  aws.String(""),
			},
			Expected: []string{
				"EddardStark",
				"CatelynStark",
				"RobStark",
			},
		},
		{
			Query:   "/House/",
			Recurse: true,
			GetParametersByPathResp: ssm.GetParametersByPathOutput{
				Parameters: HouseStark,
				NextToken:  aws.String(NextToken),
			},
			GetParametersByPathNext: ssm.GetParametersByPathOutput{
				Parameters: []*ssm.Parameter{JonSnow, DaenerysTargaryen},
				NextToken:  aws.String(""),
			},
			Expected: []string{
				"/House/Stark/EddardStark",
				"/House/Stark/CatelynStark",
				"/House/Stark/RobStark",
				"/House/Stark/JonSnow",
				"/House/Targaryen/DaenerysTargaryen",
			},
		},
	}

	for _, c := range cases {
		var p parameterstore.ParameterStore
		p.NewParameterStore()
		p.Client = mockedSSM{
			GetParametersByPathResp: c.GetParametersByPathResp,
			GetParametersByPathNext: c.GetParametersByPathNext,
			GetParametersResp:       c.GetParametersResp,
		}
		p.Cwd = parameterstore.Delimiter
		resp, err := p.List(c.Query, c.Recurse)
		if err != nil {
			t.Fatal("unexpected error", err)
		}
		if !equal(resp, c.Expected) {
			msg := fmt.Errorf("expected %v, got %v", c.Expected, resp)
			t.Fatal(msg)
		}
	}
}

func equal(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for i := 0; i < len(first); i++ {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}
