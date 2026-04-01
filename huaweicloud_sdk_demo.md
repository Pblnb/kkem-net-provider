```golang
func main() {
	setEnv()

	// ──────────────────────────────────────────
	// Step 1: 读取环境变量
	// ──────────────────────────────────────────
	ak := os.Getenv("HW_ACCESS_KEY")
	sk := os.Getenv("HW_SECRET_KEY")
	projectId := os.Getenv("HW_PROJECT_ID")

	if ak == "" || sk == "" || projectId == "" {
		fmt.Println("错误：请设置环境变量 HW_ACCESS_KEY / HW_SECRET_KEY / HW_PROJECT_ID")
		os.Exit(1)
	}

	// ──────────────────────────────────────────
	// Step 2: 构建 BasicCredentials（AK/SK 鉴权）
	// ──────────────────────────────────────────
	auth, err := basic.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		WithProjectId(projectId).
		SafeBuild()
	if err != nil {
		fmt.Printf("构建鉴权信息失败: %v\n", err)
		os.Exit(1)
	}

	// ══════════════════════════════════════════
	// 【模块一】VPC：创建 VPC
	// ══════════════════════════════════════════
	//vpcHttpClient, err := vpc.VpcClientBuilder().
	//	WithEndpoint("https://vpc.cn-north-7.myhuaweicloud.com").
	//	WithCredential(auth).
	//	SafeBuild()
	//if err != nil {
	//	fmt.Printf("构建 VPC Client 失败: %v\n", err)
	//	os.Exit(1)
	//}
	//vpcClient := vpc.NewVpcClient(vpcHttpClient)
	//
	//vpcName := "demo-vpc"
	//vpcCidr := "192.168.0.0/16"
	//vpcResp, err := vpcClient.CreateVpc(&vpcModel.CreateVpcRequest{
	//	Body: &vpcModel.CreateVpcRequestBody{
	//		Vpc: &vpcModel.CreateVpcOption{
	//			Name: &vpcName,
	//			Cidr: &vpcCidr,
	//		},
	//	},
	//})
	//if err != nil {
	//	fmt.Printf("创建 VPC 失败: %v\n", err)
	//	os.Exit(1)
	//}
	//fmt.Printf("\n✅ VPC 创建成功\n")
	//fmt.Printf("   ID   : %s\n", vpcResp.Vpc.Id)
	//fmt.Printf("   Name : %s\n", vpcResp.Vpc.Name)
	//fmt.Printf("   CIDR : %s\n", vpcResp.Vpc.Cidr)
	//fmt.Printf("   状态  : %s\n", vpcResp.Vpc.Status)

	// 后续步骤需要用到 VPC ID
	// createdVpcId := vpcResp.Vpc.Id

	// ══════════════════════════════════════════
	// 【模块二】VPCEP：创建 VPC Endpoint
	// 连接到 OBS 的 Interface 型公共终端节点服务
	// ══════════════════════════════════════════
	vpcepHttpClient, err := vpcep.VpcepClientBuilder().
		WithEndpoint("https://vpcep.cn-north-7.myhuaweicloud.com").
		WithCredential(auth).
		SafeBuild()
	if err != nil {
		fmt.Printf("构建 VPCEP Client 失败: %v\n", err)
		os.Exit(1)
	}
	//vpcepClient := vpcep.NewVpcepClient(vpcepHttpClient)
	//
	//endpointServiceId := "8900a019-e68a-49a7-a779-5fca56c1adcd"
	//vpcId := "f7b119f4-c29d-46bc-a9e5-963475edad20"
	//subnetId := "652a6fcc-8143-4e77-be32-8e1631636a25"
	//
	//epResp, err := vpcepClient.CreateEndpoint(&vpcepModel.CreateEndpointRequest{
	//	Body: &vpcepModel.CreateEndpointRequestBody{
	//		EndpointServiceId: endpointServiceId,
	//		VpcId:             vpcId,
	//		SubnetId:          &subnetId,
	//	},
	//})
	//if err != nil {
	//	fmt.Printf("创建 VPCEP Endpoint 失败: %v\n", err)
	//	os.Exit(1)
	//}
	//fmt.Printf("\n✅ VPCEP Endpoint 创建成功\n")
	//fmt.Printf("   Endpoint ID      : %s\n", *epResp.Id)
	//fmt.Printf("   Endpoint Service : %s\n", *epResp.EndpointServiceName)
	//fmt.Printf("   状态              : %s\n", *epResp.Status)
```