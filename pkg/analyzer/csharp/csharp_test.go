package csharp

import (
	"testing"
)

func TestCSharpAnalyzer(t *testing.T) {
	code := `using Microsoft.AspNetCore.Mvc;
using System.Threading.Tasks;

namespace OrderSystem.Controllers
{
    [ApiController]
    [Route("api/[controller]")]
    public class OrderController : ControllerBase
    {
        private readonly OrderDbContext _context;

        [HttpPost("create")]
        public async Task<IActionResult> CreateOrder([FromBody] Order order)
        {
            await _context.Orders.AddAsync(order);
            await _context.SaveChangesAsync();
            return Ok();
        }

        [HttpGet("{id}")]
        public IActionResult GetOrder(int id)
        {
            return Ok();
        }
    }
}
`

	analyzer := New()
	result, err := analyzer.AnalyzeFile("OrderController.cs", []byte(code))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	foundEndpoint := false
	foundEF := false

	for _, step := range result.Steps {
		if step.Type == "Endpoint" && step.Name == "CreateOrder" {
			foundEndpoint = true
			if step.Metadata["method"] != "POST" {
				t.Errorf("expected POST method, got %v", step.Metadata["method"])
			}
			if step.Metadata["route"] != "create" {
				t.Errorf("expected route 'create', got %v", step.Metadata["route"])
			}
		}
		if step.Type == "DatabaseQuery" {
			foundEF = true
		}
	}

	if !foundEndpoint {
		t.Errorf("expected to find CreateOrder endpoint step")
	}
	if !foundEF {
		t.Errorf("expected to find EF operation step")
	}

	if len(result.Links) == 0 {
		t.Errorf("expected links between method and EF operations")
	}
}

func TestRazorAnalyzer(t *testing.T) {
	razor := `@page "/dashboard"
@inject OrderService OrderSvc

<h3>Dashboard</h3>

@code {
    protected override async Task OnInitializedAsync()
    {
        await OrderSvc.LoadOrders();
    }
}
`
	analyzer := New()
	result, err := analyzer.AnalyzeFile("Dashboard.razor", []byte(razor))
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if len(result.Steps) == 0 {
		t.Fatalf("expected steps from Razor file")
	}

	foundPage := false
	for _, s := range result.Steps {
		if s.Metadata["route"] == "/dashboard" {
			foundPage = true
		}
	}
	if !foundPage {
		t.Errorf("expected /dashboard route from razor page")
	}
}
