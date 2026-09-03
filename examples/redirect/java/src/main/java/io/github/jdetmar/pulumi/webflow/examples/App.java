package io.github.jdetmar.pulumi.webflow.examples;

import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import io.github.jdetmar.pulumi.webflow.Redirect;
import io.github.jdetmar.pulumi.webflow.RedirectArgs;

import java.util.List;
import java.util.ArrayList;
import java.util.stream.Collectors;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            // Get configuration values
            var config = ctx.config();
            var siteId = config.requireSecret("siteId");

            // Webflow redirects are always permanent (HTTP 301). The deprecated
            // statusCode input is ignored by the provider, so it is simply omitted here.

            // Example 1: Content Move Redirect - Best for content moves
            // Permanent redirects preserve SEO value when content has moved
            var permanentRedirect = new Redirect("old-blog-to-new-blog",
                RedirectArgs.builder()
                    .siteId(siteId)
                    .sourcePath("/blog/old-article")
                    .destinationPath("/blog/articles/updated-article")
                    .build());

            // Example 2: Campaign Redirect - Point a short path at the current campaign page
            // Changing destinationPath later updates the redirect in place
            var campaignRedirect = new Redirect("campaign-landing-page",
                RedirectArgs.builder()
                    .siteId(siteId)
                    .sourcePath("/old-campaign")
                    .destinationPath("/new-campaign-2025")
                    .build());

            // Example 3: External Redirect - Redirect to another domain
            // Useful for partner links or moved subdomains
            var externalRedirect = new Redirect("external-partner-link",
                RedirectArgs.builder()
                    .siteId(siteId)
                    .sourcePath("/partner")
                    .destinationPath("https://partner-site.com")
                    .build());

            // Example 4: Bulk Redirects using Loop
            // Efficient pattern for migrating multiple URLs at once
            var redirectMappings = List.of(
                new String[]{"/product-a", "/products/product-a"},
                new String[]{"/product-b", "/products/product-b"},
                new String[]{"/product-c", "/products/product-c"}
            );

            var bulkRedirects = new ArrayList<Redirect>();
            for (int i = 0; i < redirectMappings.size(); i++) {
                var mapping = redirectMappings.get(i);
                var redirect = new Redirect("bulk-redirect-" + i,
                    RedirectArgs.builder()
                        .siteId(siteId)
                        .sourcePath(mapping[0])
                        .destinationPath(mapping[1])
                        .build());
                bulkRedirects.add(redirect);
            }

            // Export values for reference
            ctx.export("deployedSiteId", siteId);
            ctx.export("permanentRedirectId", permanentRedirect.id());
            ctx.export("campaignRedirectId", campaignRedirect.id());
            ctx.export("externalRedirectId", externalRedirect.id());
            ctx.export("bulkRedirectIds", Output.all(bulkRedirects.stream()
                .map(Redirect::id)
                .collect(Collectors.toList())));
        });
    }
}
