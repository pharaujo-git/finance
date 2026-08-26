using System.Globalization;

namespace FinanceTracker.Api.Common;

/// <summary>
/// Resolves the TCP port the hosting platform assigns through <c>PORT</c>.
/// Kestrel binds it as a plain socket on purpose: Render terminates TLS at its edge proxy and
/// forwards cleartext over the container's loopback network, so the app must not expect HTTPS here.
/// </summary>
public static class ServerBinding
{
    public const string PortVariable = "PORT";

    public const int DefaultPort = 8080;

    private const int MaxPort = 65535;

    public static int ResolvePort(IConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);

        return int.TryParse(configuration[PortVariable], CultureInfo.InvariantCulture, out var port)
               && port is > 0 and <= MaxPort
            ? port
            : DefaultPort;
    }
}
