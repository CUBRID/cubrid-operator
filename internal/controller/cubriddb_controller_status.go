package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	cubridv1 "github.com/cubrid/cubrid-operator/api/v1"
	cms "github.com/cubrid/cubrid-operator/pkg/cms"
	DEF "github.com/cubrid/cubrid-operator/pkg/config"
	"github.com/cubrid/cubrid-operator/pkg/util"
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
	var listStatus []string
	listStatus = make([]string, 0)

	currentStatusWithoutLastUpdated := cubriddb.Status.DeepCopy()

	if cubriddb.IsHAEnabled() {
		var responseMap map[string]interface{}
		var podUrl string
		var retToken string
		var isSuccess bool = false

		httpUrls, errCode = util.CreatePodFullURLs(ctx, client, cubriddb.Name, cubriddb.Namespace, DEF.CMS_PORT)
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
				cubriddblog.Error(errCode, "Error checking response status")
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
			cubriddb.Status.NodeLists = []string{fmt.Sprintf("%s: %s", cubriddb.Name, DEF.HAMODE_UNKONW)}
			cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}
			cubriddb.Status.CurrentMaster = DEF.HAMODE_UNKONW
		} else {

			cubriddb.Status.HaMode = DEF.HAMODE_ON
			cubriddb.Status.NodeLists = listStatus
			cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}

			for _, nodeStatus := range listStatus {
				parts := strings.Split(nodeStatus, ": ")
				if len(parts) == 2 && parts[1] == DEF.HAMODE_MASTER {
					cubriddb.Status.CurrentMaster = parts[0]
					break
				}
			}
		}
	} else {
		cubriddb.Status.HaMode = DEF.HAMODE_OFF
		cubriddb.Status.LastUpdated = metav1.Time{Time: time.Now()}
		cubriddb.Status.NodeLists = []string{fmt.Sprintf("%s", DEF.HAMODE_STANDALONE)}
		cubriddb.Status.CurrentMaster = cubriddb.Name
	}

	newStatusWithoutLastUpdated := cubriddb.Status.DeepCopy()
	newStatusWithoutLastUpdated.LastUpdated = metav1.Time{}

	currentStatusWithoutLastUpdated.LastUpdated = metav1.Time{}

	if currentStatusWithoutLastUpdated.NodeLists == nil {
		currentStatusWithoutLastUpdated.NodeLists = []string{}
	}
	if newStatusWithoutLastUpdated.NodeLists == nil {
		newStatusWithoutLastUpdated.NodeLists = []string{}
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

// parseHANodesStatus parses the HA status and returns a slice of node status strings in order
func parseHANodesStatus(result map[string]interface{}) ([]string, error) {
	nodes := make(map[string]string)

	// Add all existing nodes
	for key, value := range result {
		if strings.HasPrefix(key, "node") && !strings.HasSuffix(key, "_state") {
			if nodeValue, ok := value.(string); ok {
				stateKey := key + "_state"
				if stateValue, ok := result[stateKey].(string); ok {
					nodeName := strings.Split(nodeValue, ".")[0]
					nodes[nodeName] = stateValue
				}
			}
		}
	}

	// Create ordered slice of node status strings
	var orderedNodes []string

	// Helper function to get sorted nodes by role
	getSortedNodesByRole := func(role string) []string {
		var roleNodes []string
		for node, state := range nodes {
			if strings.ToLower(state) == strings.ToLower(role) {
				roleNodes = append(roleNodes, node)
			}
		}
		sort.Strings(roleNodes)
		return roleNodes
	}

	// First, add master nodes
	for _, node := range getSortedNodesByRole(DEF.HAMODE_MASTER) {
		orderedNodes = append(orderedNodes, fmt.Sprintf("%s: %s", node, nodes[node]))
	}

	// Then, add slave nodes in alphabetical order
	for _, node := range getSortedNodesByRole(DEF.HAMODE_SLAVE) {
		orderedNodes = append(orderedNodes, fmt.Sprintf("%s: %s", node, nodes[node]))
	}

	// Finally, add replica nodes in alphabetical order
	for _, node := range getSortedNodesByRole(DEF.HAMODE_REPLICA) {
		orderedNodes = append(orderedNodes, fmt.Sprintf("%s: %s", node, nodes[node]))
	}

	return orderedNodes, nil
}

func (r *CubridDBReconciler) loginToCMServer(httpURL, id, passwd, version string) (string, error) {
	loginCmd := createLoginCommand(id, passwd, version)

	resp, err := cms.SendCommand(loginCmd, httpURL)
	if err != nil {
		return "", fmt.Errorf("error sending login command: %v", err)
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				cubriddblog.Error(err, "Error closing response body")
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

	resp, err := cms.SendCommand(command, httpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to send ha_status command: %v", err)
	}

	defer func() {
		if resp != nil && resp.Body != nil {
			if err := resp.Body.Close(); err != nil {
				cubriddblog.Error(err, "Error closing response body")
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

func createLoginCommand(id, password, clientver string) *cms.CMSCommand {
	return cms.CreateLoginCommand(id, password, clientver)
}

func createStatusCommand(token string) (*cms.CMSCommand, error) {
	if token == "" {
		return nil, fmt.Errorf("invalid empty token")
	}
	return cms.CreateHAStatusCommand(token), nil
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
