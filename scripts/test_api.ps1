param (
    [Parameter(Mandatory=$true)]
    [string]$BaseURL
)

$BaseURL = $BaseURL.TrimEnd("/")

function Test-Endpoint {
    param($Method, $Path, $Body, $Token)
    
    $url = "$BaseURL$Path"
    Write-Host "`nTesting $Method $Path..." -ForegroundColor Cyan
    
    $headers = @{
        "Content-Type" = "application/json"
    }
    if ($Token) {
        $headers["Authorization"] = "Bearer $Token"
    }
    
    try {
        if ($Body) {
            $jsonBody = $Body | ConvertTo-Json
            $response = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers -Body $jsonBody
        } else {
            $response = Invoke-RestMethod -Uri $url -Method $Method -Headers $headers
        }
        Write-Host "Success!" -ForegroundColor Green
        return $response
    } catch {
        Write-Host "Failed: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $errBody = $reader.ReadToEnd()
            Write-Host "Error Body: $errBody" -ForegroundColor Yellow
        }
        return $null
    }
}

# 1. Health Check
Test-Endpoint -Method "GET" -Path "/health"

# 2. Register Buyer
$buyerEmail = "buyer_$(Get-Date -Format 'HHmmss')@test.com"
$regRes = Test-Endpoint -Method "POST" -Path "/auth/register" -Body @{
    email = $buyerEmail
    password = "password123"
    role = "buyer"
}

# 3. Login Buyer
if ($regRes) {
    $loginRes = Test-Endpoint -Method "POST" -Path "/auth/login" -Body @{
        email = $buyerEmail
        password = "password123"
    }
    if ($loginRes -and $loginRes.access_token) {
        $buyerToken = $loginRes.access_token
        Write-Host "Buyer Token obtained." -ForegroundColor Green
    }
}

# 4. List Products (Public)
Test-Endpoint -Method "GET" -Path "/products"

# 5. Register Vendor
$vendorEmail = "vendor_$(Get-Date -Format 'HHmmss')@test.com"
$vRegRes = Test-Endpoint -Method "POST" -Path "/auth/register" -Body @{
    email = $vendorEmail
    password = "password123"
    role = "vendor"
}

# 6. Login Vendor
if ($vRegRes) {
    $vLoginRes = Test-Endpoint -Method "POST" -Path "/auth/login" -Body @{
        email = $vendorEmail
        password = "password123"
    }
}

Write-Host "`nTesting Complete!" -ForegroundColor Gray
