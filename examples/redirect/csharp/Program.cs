using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading.Tasks;
using Pulumi;
using Community.Pulumi.Webflow;

class Program
{
    static Task<int> Main() => Deployment.RunAsync(() =>
    {
        // Create a Pulumi config object
        var config = new Pulumi.Config();

        // Get configuration values
        var siteId = config.RequireSecret("siteId");

        // Webflow redirects are always permanent (HTTP 301). The deprecated
        // StatusCode input is ignored by the provider, so it is simply omitted here.

        // Example 1: Content Move Redirect - Best for content moves
        // Permanent redirects preserve SEO value when content has moved
        var permanentRedirect = new Redirect("old-blog-to-new-blog", new RedirectArgs
        {
            SiteId = siteId,
            SourcePath = "/blog/old-article",
            DestinationPath = "/blog/articles/updated-article",
        });

        // Example 2: Campaign Redirect - Point a short path at the current campaign page
        // Changing DestinationPath later updates the redirect in place
        var campaignRedirect = new Redirect("campaign-landing-page", new RedirectArgs
        {
            SiteId = siteId,
            SourcePath = "/old-campaign",
            DestinationPath = "/new-campaign-2025",
        });

        // Example 3: External Redirect - Redirect to another domain
        // Useful for partner links or moved subdomains
        var externalRedirect = new Redirect("external-partner-link", new RedirectArgs
        {
            SiteId = siteId,
            SourcePath = "/partner",
            DestinationPath = "https://partner-site.com",
        });

        // Example 4: Bulk Redirects using Loop
        // Efficient pattern for migrating multiple URLs at once
        var redirectMappings = new[]
        {
            new { Old = "/product-a", New = "/products/product-a" },
            new { Old = "/product-b", New = "/products/product-b" },
            new { Old = "/product-c", New = "/products/product-c" },
        };

        var bulkRedirects = new List<Redirect>();
        for (int i = 0; i < redirectMappings.Length; i++)
        {
            var mapping = redirectMappings[i];
            var redirect = new Redirect($"bulk-redirect-{i}", new RedirectArgs
            {
                SiteId = siteId,
                SourcePath = mapping.Old,
                DestinationPath = mapping.New,
            });
            bulkRedirects.Add(redirect);
        }

        // Export the redirect resources for reference
        return new Dictionary<string, object?>
        {
            ["deployedSiteId"] = siteId,
            ["permanentRedirectId"] = permanentRedirect.Id,
            ["campaignRedirectId"] = campaignRedirect.Id,
            ["externalRedirectId"] = externalRedirect.Id,
            ["bulkRedirectIds"] = Output.All(bulkRedirects.Select(r => r.Id).ToArray()),
        };
    });
}
