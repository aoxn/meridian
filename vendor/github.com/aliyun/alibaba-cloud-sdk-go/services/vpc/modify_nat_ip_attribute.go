package vpc

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// ModifyNatIpAttribute invokes the vpc.ModifyNatIpAttribute API synchronously
func (client *Client) ModifyNatIpAttribute(request *ModifyNatIpAttributeRequest) (response *ModifyNatIpAttributeResponse, err error) {
	response = CreateModifyNatIpAttributeResponse()
	err = client.DoAction(request, response)
	return
}

// ModifyNatIpAttributeWithChan invokes the vpc.ModifyNatIpAttribute API asynchronously
func (client *Client) ModifyNatIpAttributeWithChan(request *ModifyNatIpAttributeRequest) (<-chan *ModifyNatIpAttributeResponse, <-chan error) {
	responseChan := make(chan *ModifyNatIpAttributeResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.ModifyNatIpAttribute(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// ModifyNatIpAttributeWithCallback invokes the vpc.ModifyNatIpAttribute API asynchronously
func (client *Client) ModifyNatIpAttributeWithCallback(request *ModifyNatIpAttributeRequest, callback func(response *ModifyNatIpAttributeResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *ModifyNatIpAttributeResponse
		var err error
		defer close(result)
		response, err = client.ModifyNatIpAttribute(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// ModifyNatIpAttributeRequest is the request struct for api ModifyNatIpAttribute
type ModifyNatIpAttributeRequest struct {
	*requests.RpcRequest
	ResourceOwnerId      requests.Integer `position:"Query" name:"ResourceOwnerId"`
	NatIpName            string           `position:"Query" name:"NatIpName"`
	ClientToken          string           `position:"Query" name:"ClientToken"`
	NatIpDescription     string           `position:"Query" name:"NatIpDescription"`
	DryRun               requests.Boolean `position:"Query" name:"DryRun"`
	NatIpId              string           `position:"Query" name:"NatIpId"`
	ResourceOwnerAccount string           `position:"Query" name:"ResourceOwnerAccount"`
	OwnerAccount         string           `position:"Query" name:"OwnerAccount"`
	OwnerId              requests.Integer `position:"Query" name:"OwnerId"`
}

// ModifyNatIpAttributeResponse is the response struct for api ModifyNatIpAttribute
type ModifyNatIpAttributeResponse struct {
	*responses.BaseResponse
	RequestId string `json:"RequestId" xml:"RequestId"`
}

// CreateModifyNatIpAttributeRequest creates a request to invoke ModifyNatIpAttribute API
func CreateModifyNatIpAttributeRequest() (request *ModifyNatIpAttributeRequest) {
	request = &ModifyNatIpAttributeRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Vpc", "2016-04-28", "ModifyNatIpAttribute", "vpc", "openAPI")
	request.Method = requests.POST
	return
}

// CreateModifyNatIpAttributeResponse creates a response to parse from ModifyNatIpAttribute response
func CreateModifyNatIpAttributeResponse() (response *ModifyNatIpAttributeResponse) {
	response = &ModifyNatIpAttributeResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}
