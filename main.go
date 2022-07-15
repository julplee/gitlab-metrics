package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"sort"
	"time"
)

type JsonTeamsData struct {
	Teams []Team `json:"teams"`
}

type Team struct {
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

type Member struct {
	Name string `json:"name"`
}

type JsonMergeRequestsData struct {
	Data Data `json:"data"`
}

type Data struct {
	Project Project `json:"project"`
}

type Project struct {
	Name          string        `json:"name"`
	MergeRequests MergeRequests `json:"mergeRequests"`
}

type MergeRequests struct {
	Nodes []MergeRequest `json:"nodes"`
}

/* `json:""` */
type MergeRequest struct {
	Id                   string  `json:"id"`
	Title                string  `json:"title"`
	Author               Author  `json:"author"`
	UserDiscussionsCount int     `json:"userDiscussionsCount"`
	UserNotesCount       int     `json:"userNotesCount"`
	State                string  `json:"state"`
	CreatedAt            string  `json:"createdAt"`
	MergedAt             string  `json:"mergedAt"`
	Commits              Commits `json:"commits"`
	Url                  string  `json:"webUrl"`
}

type Author struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Commits struct {
	Nodes []Commit `json:"nodes"`
}

type Commit struct {
	AuthoredDate string `json:"authoredDate"`
}

type AnalyzedMergeRequest struct {
	Id                                    string
	Author                                string
	UserDiscussionsCount                  int
	UserNotesCount                        int
	CommitCount                           int
	Url                                   string
	ReviewTimeInDays                      float64
	CommitToMergeRequestionCreationInDays float64
	TotalCycleTimeInDays                  float64
}

func main() {
	var analyzedMergeRequestsByTeam map[string]map[int][]AnalyzedMergeRequest
	analyzedMergeRequestsByTeam = make(map[string]map[int][]AnalyzedMergeRequest)

	var mergeRequestsData JsonMergeRequestsData

	gitLabDataFiles, shouldReturn := readFiles("./data")
	if shouldReturn {
		return
	}

	gitLabTeamMembersFiles, shouldReturn := readFiles("./team-members")
	if shouldReturn {
		return
	}

	var membersTeam map[string]string

	keys := make([]string, 0, len(gitLabDataFiles))
	for k := range gitLabDataFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := gitLabDataFiles[k]
		if v.IsDir() {
			continue
		}

		index := GetFileIndex(v.Name())
		if gitLabTeamMembersFiles[index] != nil {
			membersTeam = ReadTeamsDataJson(gitLabTeamMembersFiles[index].Name(), analyzedMergeRequestsByTeam)
		}

		jsonMergeRequests, err := os.Open("./data/" + v.Name())

		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Successfully Opened JSON file " + v.Name())

		byteValue, _ := ioutil.ReadAll(jsonMergeRequests)
		json.Unmarshal(byteValue, &mergeRequestsData)

		layoutAt := "2006-01-02T15:04:05Z"
		layoutDate := "2006-01-02T15:04:05Z07:00"

		for _, mergeRequest := range mergeRequestsData.Data.Project.MergeRequests.Nodes {

			timeCreatedAt, err := time.Parse(layoutAt, mergeRequest.CreatedAt)
			if err != nil {
				fmt.Println(err)
			}

			timeMergedAt, err := time.Parse(layoutAt, mergeRequest.MergedAt)
			if err != nil {
				fmt.Println(err)
			}

			firstActivityDate := timeCreatedAt
			for k := 0; k < len(mergeRequest.Commits.Nodes); k++ {
				commitDate, err := time.Parse(layoutDate, mergeRequest.Commits.Nodes[k].AuthoredDate)
				if err != nil {
					fmt.Println(err)
				}

				if firstActivityDate.After(commitDate) {
					firstActivityDate = commitDate
				}

				if firstActivityDate.After(timeMergedAt) {
					fmt.Println("ERROR commit date is after merge date" + mergeRequest.Url)
					os.Exit(1)
				}
			}

			_, week := timeMergedAt.ISOWeek()

			cycleTime := timeMergedAt.Sub(firstActivityDate).Hours()
			totalCycleTimeDay := cycleTime / 24

			reviewTime := timeMergedAt.Sub(timeCreatedAt).Hours()
			reviewTimeDay := reviewTime / 24

			commitToMergeRequestionCreationTime := timeCreatedAt.Sub(firstActivityDate).Hours()
			commitToMergeRequestionCreationTimeDay := commitToMergeRequestionCreationTime / 24

			team := membersTeam[mergeRequest.Author.Name]

			if _, ok := analyzedMergeRequestsByTeam[team]; ok {
				var analyzedMergeRequest AnalyzedMergeRequest
				analyzedMergeRequest.Id = mergeRequest.Id
				analyzedMergeRequest.Author = mergeRequest.Author.Name
				analyzedMergeRequest.UserDiscussionsCount = mergeRequest.UserDiscussionsCount
				analyzedMergeRequest.UserNotesCount = mergeRequest.UserNotesCount
				analyzedMergeRequest.CommitCount = len(mergeRequest.Commits.Nodes)
				analyzedMergeRequest.ReviewTimeInDays = reviewTimeDay
				analyzedMergeRequest.CommitToMergeRequestionCreationInDays = commitToMergeRequestionCreationTimeDay
				analyzedMergeRequest.TotalCycleTimeInDays = totalCycleTimeDay
				analyzedMergeRequest.Url = mergeRequest.Url

				analyzedMergeRequestsByTeam[team][week] = append(analyzedMergeRequestsByTeam[team][week], analyzedMergeRequest)
			} else {
				fmt.Println("ERROR team not found for " + mergeRequest.Author.Name)
			}
		}

		for keyTeamName, valueAnalyzedMergeRequestsByWeek := range analyzedMergeRequestsByTeam {
			fmt.Printf("Team: %s\n", keyTeamName)
			for keyWeek, valueAnalyzedMergeRequests := range valueAnalyzedMergeRequestsByWeek {
				fmt.Printf("Week: %d ->\n", keyWeek)
				for _, analyzedMergeRequest := range valueAnalyzedMergeRequests {
					fmt.Printf("\t%+v\n", analyzedMergeRequest)
				}
			}
		}

		// defer the closing of our jsonFile so that we can parse it later on
		defer jsonMergeRequests.Close()
	}

}

func readFiles(filePath string) (map[string]fs.FileInfo, bool) {
	dataDir, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
		return nil, true
	}

	dataFiles, err := dataDir.Readdir(0)
	if err != nil {
		fmt.Println(err)
		return nil, true
	}

	defer dataDir.Close()

	files := make(map[string]fs.FileInfo)
	for _, v := range dataFiles {
		if v.Name() == "README.md" {
			continue
		}

		index := GetFileIndex(v.Name())

		files[index] = v
	}

	return files, false
}

func GetFileIndex(fileName string) string {
	length := len(fileName)

	return fileName[length-7 : length-5]
}

func ReadTeamsDataJson(gitLabTeamMembersFileName string, analyzedMergeRequestsByTeam map[string]map[int][]AnalyzedMergeRequest) map[string]string {
	var teamsData JsonTeamsData

	jsonTeams, err := os.Open("./team-members/" + gitLabTeamMembersFileName)

	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened JSON file " + gitLabTeamMembersFileName)

	bytesValue, _ := ioutil.ReadAll(jsonTeams)
	json.Unmarshal(bytesValue, &teamsData)

	var membersTeam map[string]string
	membersTeam = make(map[string]string)

	for _, team := range teamsData.Teams {
		analyzedMergeRequestsByTeam[team.Name] = make(map[int][]AnalyzedMergeRequest)

		for _, member := range team.Members {
			membersTeam[member.Name] = team.Name
		}
	}

	jsonTeams.Close()

	return membersTeam
}
