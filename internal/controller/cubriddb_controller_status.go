package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	"github.com/cubrid/cubrid-operator/pkg"
	DEF "github.com/cubrid/cubrid-operator/pkg/define"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeStatus struct {
	node  string
	state string
}

func (r *CubridDBReconciler) UpdateCubridDBStatus(
	ctx context.Context,
	client client.Client,
	cubriddb *cubridv1.CubridDB,
) (ctrl.Result, error) {

	var token string
	var isError bool = false
	var errCode error
	var errorMessage error
	var httpUrls []string
	var listStatus map[string]string
	listStatus = make(map[string]string)

	currentStatusWithoutLastUpdated := cubriddb.Status.DeepCopy()

	if cubriddb.IsHAEnabled() {
		var responseMap map[string]interface{}
		var podUrl string
		var retToken string
		var isSuccess bool = false

		httpUrls, errCode = pkg.CreatePodFullURLs(ctx, client, cubriddb.Name, cubriddb.Namespace, DEF.CMS_PORT)
		if errCode != nil {
			errorMessage = fmt.Errorf("error CreatePodFullURLs: %v", errCode)
			goto End
		}

		for _, url := range httpUrls {
			retToken, errCode = r.loginToCMServer(url, DEF.CMS_ID, DEF.CMS_PW, DEF.CMS_VERSION)
			if errCode != nil {
				continue
			}

			podUrl = url
			token = retToken

			responseMap, errCode = r.requestHAStatus(podUrl, token)
			if errCode != nil {
				continue
			}

			status, errCode := r.isResponseStatus(responseMap)
			if errCode != nil {
				fmt.Printf("=====>>> %v", errCode)
				continue
			}

			isSuccess = status
			if isSuccess {
				break
			}
		}

		if !isSuccess {
			errorMessage = fmt.Errorf("failed to login and ha_status : %v", errCode)
			isError = true
			goto End
		}

		listStatus, errCode = parseHANodesStatus(responseMap)
		if errCode != nil {
			errorMessage = fmt.Errorf("failed to parse HA Mode status: %v", errCode)
			isError = true
			goto End
		}

	End:
		if isError {
			cubriddb.Status.HaMode = DEF.HAMODE_ON
			cubriddb.Status.NodeLists = make(map[string]string)
			cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}
			cubriddb.Status.CurrentMaster = DEF.HAMODE_UNKONW
		} else {

			cubriddb.Status.HaMode = DEF.HAMODE_ON
			cubriddb.Status.NodeLists = listStatus
			cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}

			for node, state := range listStatus {
				if state == DEF.HAMODE_MASTER {
					cubriddb.Status.CurrentMaster = node
					break
				}
			}
		}
	} else {
		cubriddb.Status.HaMode = DEF.HAMODE_OFF
		cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}
		cubriddb.Status.NodeLists = map[string]string{cubriddb.Name: DEF.HAMODE_STANDALONE}
		cubriddb.Status.CurrentMaster = cubriddb.Name
	}

	newStatusWithoutLastUpdated := cubriddb.Status.DeepCopy()
	newStatusWithoutLastUpdated.LastUpdated = metav1.Time{}

	currentStatusWithoutLastUpdated.LastUpdated = metav1.Time{}

	if currentStatusWithoutLastUpdated.NodeLists == nil {
		currentStatusWithoutLastUpdated.NodeLists = map[string]string{}
	}
	if newStatusWithoutLastUpdated.NodeLists == nil {
		newStatusWithoutLastUpdated.NodeLists = map[string]string{}
	}

	if reflect.DeepEqual(currentStatusWithoutLastUpdated, newStatusWithoutLastUpdated) {
		cubriddblog.V(1).Info("Status is unchanged (excluding LastUpdated), skipping update",
			"name", cubriddb.Name, "namespace", cubriddb.Namespace)
		return ctrl.Result{}, nil
	}

	if err := r.Status().Update(ctx, cubriddb); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update CubridDB status: %v", err)
	}

	if isError {
		return ctrl.Result{}, errorMessage
	} else {
		return ctrl.Result{}, nil
	}
}

func parseHANodesStatus(responseMap map[string]interface{}) (map[string]string, error) {
	replicationStatus := make(map[string]string)

	nodePattern := regexp.MustCompile(`^node[A-Za-z]+$`)

	for key, value := range responseMap {
		if match := nodePattern.FindStringSubmatch(key); match != nil {
			node := fmt.Sprintf("%v", value)

			parts := strings.Split(node, ".")
			if len(parts) > 1 {
				node = parts[0]
			}

			stateKey := fmt.Sprintf("%s_state", key)
			state, ok := responseMap[stateKey].(string)
			if !ok {
				fmt.Printf("State key %s not found for node %s\n", stateKey, node)
				continue
			}
			repCase := cases.Title(language.English)
			replicationStatus[node] = repCase.String(state)
		}
	}

	if len(replicationStatus) == 0 {
		return nil, fmt.Errorf("no valid nodes found in ha_status response")
	}

	priority := map[string]int{DEF.HAMODE_MASTER: 0, DEF.HAMODE_SLAVE: 1, DEF.HAMODE_REPLICA: 2}

	sortedNodes := make([]nodeStatus, 0, len(replicationStatus))
	for node, state := range replicationStatus {
		sortedNodes = append(sortedNodes, nodeStatus{node: node, state: state})
	}

	sort.Slice(sortedNodes, func(i, j int) bool {
		numPattern := regexp.MustCompile(`\d+`)
		iNumStr := numPattern.FindString(sortedNodes[i].node)
		jNumStr := numPattern.FindString(sortedNodes[j].node)

		iNum, _ := strconv.Atoi(iNumStr)
		jNum, _ := strconv.Atoi(jNumStr)

		if iNum != jNum {
			return iNum < jNum
		}

		return priority[sortedNodes[i].state] < priority[sortedNodes[j].state]
	})

	sortedReplicationStatus := make(map[string]string)
	for _, ns := range sortedNodes {
		sortedReplicationStatus[ns.node] = ns.state
	}

	return sortedReplicationStatus, nil
}

func (r *CubridDBReconciler) loginToCMServer(httpURL, id, passwd, version string) (string, error) {
	loginCmd := createLoginCommand(id, passwd, version)

	resp, err := pkg.SendCommand(loginCmd, httpURL)
	if err != nil {
		return "", fmt.Errorf("error sending login command: %v", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				fmt.Printf("error closing response body: %v", err)
			}
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d from login API", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse login response: %v", err)
	}

	token, ok := response["token"].(string)
	if !ok {
		return "", fmt.Errorf("token not found in login response")
	}

	return token, nil
}

func (r *CubridDBReconciler) requestHAStatus(httpURL, token string) (map[string]interface{}, error) {
	command, err := createStatusCommand(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create ha_status command: %v", err)
	}

	resp, err := pkg.SendCommand(command, httpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send ha_status command: %v", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				fmt.Printf("error closing response body: %v", err)
			}
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d from ha_status API", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ha_status response: %v", err)
	}

	return responseMap, nil
}

func createLoginCommand(id, password, clientver string) *pkg.Command {
	command := &pkg.Command{
		Task: DEF.CMS_CMD_LOGIN,
		Extra: map[string]interface{}{
			"id":        id,
			"password":  password,
			"clientver": clientver,
		},
	}

	jsonData, _ := json.Marshal(command)
	fmt.Println("command: ", string(jsonData))

	return command
}

func createStatusCommand(token string) (*pkg.Command, error) {
	if token == "" {
		return nil, fmt.Errorf("invalid empty token")
	}

	extra := map[string]interface{}{
		"token": token,
	}

	// 상태 명령어 생성
	builder := pkg.NewCommandBuilder(token)
	return builder.CreateCommand(DEF.CMS_CMD_HA_STATUS, extra)
}

func (r *CubridDBReconciler) getCredentialsFromSecret(ctx context.Context, secretName, namespace string) (string, string, string, error) {
	secret := &corev1.Secret{}

	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretName}, secret)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get Secret %s in namespace %s: %v", secretName, namespace, err)
	}

	username, usernameExists := secret.Data["username"]
	password, passwordExists := secret.Data["password"]
	port, portExists := secret.Data["port"]

	if !usernameExists || !passwordExists || !portExists {
		return "", "", "", fmt.Errorf("username or password not found in Secret %s", secretName)
	}

	return string(username), string(password), string(port), nil
}

func (r *CubridDBReconciler) isResponseStatus(response map[string]interface{}) (bool, error) {
	status, ok := response["status"].(string)
	if !ok {
		return false, fmt.Errorf("missing or invalid 'status' field")
	}
	if status != "success" {
		note, ok := response["note"].(string)
		if ok && note != "" {
			return false, fmt.Errorf("response status failure: %s", note)
		}
		return false, fmt.Errorf("response status failure: unknown reason")
	}

	return true, nil
}
