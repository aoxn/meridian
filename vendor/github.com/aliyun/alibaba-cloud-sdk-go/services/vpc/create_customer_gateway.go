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

// CreateCustomerGateway invokes the vpc.CreateCustomerGateway API synchronously
func (client *Client) CreateCustomerGateway(request *CreateCustomerGatewayRequest) (response *CreateCustomerGatewayResponse, err error) {
	response = CreateCreateCustomerGatewayResponse()
	err = client.DoAction(request, response)
	return
}

// CreateCustomerGatewayWithChan invokes the vpc.CreateCustomerGateway API asynchronously
func (client *Client) CreateCustomerGatewayWithChan(request *CreateCustomerGatewayRequest) (<-chan *CreateCustomerGatewayResponse, <-chan error) {
	responseChan := make(chan *CreateCustomerGatewayResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.CreateCustomerGateway(request)
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

// CreateCustomerGatewayWithCallback invokes the vpc.CreateCustomerGateway API asynchronously
func (client *Client) CreateCustomerGatewayWithCallback(request *CreateCustomerGatewayRequest, callback func(response *CreateCustomerGatewayResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *CreateCustomerGatewayResponse
		var err error
		defer close(result)
		response, err = client.CreateCustomerGateway(request)
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

// CreateCustomerGatewayRequest is the request struct for api CreateCustomerGateway
type CreateCustomerGatewayRequest struct {
	*requests.RpcRequest
	IpAddress            string                       `position:"Query" name:"IpAddress"`
	AuthKey              string                       `position:"Query" name:"AuthKey"`
	ResourceOwnerId      requests.Integer             `position:"Query" name:"ResourceOwnerId"`
	ClientToken          string                       `position:"Query" name:"ClientToken"`
	Description          string                       `position:"Query" name:"Description"`
	ResourceGroupId      string                       `position:"Query" name:"ResourceGroupId"`
	ResourceOwnerAccount string                       `position:"Query" name:"ResourceOwnerAccount"`
	OwnerAccount         string                       `position:"Query" name:"OwnerAccount"`
	OwnerId              requests.Integer             `position:"Query" name:"OwnerId"`
	Tags                 *[]CreateCustomerGatewayTags `position:"Query" name:"Tags"  type:"Repeated"`
	Name                 string                       `position:"Query" name:"Name"`
	Asn                  string                       `position:"Query" name:"Asn"`
}

// CreateCustomerGatewayTags is a repeated param struct in CreateCustomerGatewayRequest
type CreateCustomerGatewayTags struct {
	Value string `name:"Value"`
	Key   string `name:"Key"`
}

// CreateCustomerGatewayResponse is the response struct for api CreateCustomerGateway
type CreateCustomerGatewayResponse struct {
	*responses.BaseResponse
	RequestId         string `json:"RequestId" xml:"RequestId"`
	IpAddress         string `json:"IpAddress" xml:"IpAddress"`
	Description       string `json:"Description" xml:"Description"`
	CustomerGatewayId string `json:"CustomerGatewayId" xml:"CustomerGatewayId"`
	CreateTime        int64  `json:"CreateTime" xml:"CreateTime"`
	Name              string `json:"Name" xml:"Name"`
}

// CreateCreateCustomerGatewayRequest creates a request to invoke CreateCustomerGateway API
func CreateCreateCustomerGatewayRequest() (request *CreateCustomerGatewayRequest) {
	request = &CreateCustomerGatewayRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Vpc", "2016-04-28", "CreateCustomerGateway", "vpc", "openAPI")
	request.Method = requests.POST
	return
}

// CreateCreateCustomerGatewayResponse creates a response to parse from CreateCustomerGateway response
func CreateCreateCustomerGatewayResponse() (response *CreateCustomerGatewayResponse) {
	response = &CreateCustomerGatewayResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}
