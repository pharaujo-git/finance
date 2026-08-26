using FinanceTracker.Api.Common;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;

namespace FinanceTracker.Api.Controllers;

/// <summary>Base for every authenticated endpoint: fixes the route prefix and exposes the caller's id.</summary>
[ApiController]
[Authorize]
[Route("api/[controller]")]
[Produces("application/json")]
public abstract class ApiControllerBase : ControllerBase
{
    protected Guid UserId => User.GetUserId();
}
