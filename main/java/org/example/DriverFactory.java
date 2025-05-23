package org.example;

import org.openqa.selenium.WebDriver;
import org.openqa.selenium.chrome.ChromeDriver;
import org.openqa.selenium.chrome.ChromeOptions;
import org.openqa.selenium.firefox.FirefoxDriver;
import org.openqa.selenium.firefox.FirefoxOptions;
import org.openqa.selenium.remote.RemoteWebDriver;

import java.net.MalformedURLException;
import java.net.URI;
import java.net.URISyntaxException;
import java.net.URL;
import java.util.Optional;

public class DriverFactory {

    private static final String grid_url = System.getenv("GRID_URL");

    private static final String browser = Optional
            .ofNullable(System.getenv("BROWSER"))
            .orElse("chrome");

    public static WebDriver getDriver() {
        if (grid_url != null) {
            return getRemoteDriver(browser);
        } else {
            return getLocalDriver(browser);
        }
    }

    private static WebDriver getRemoteDriver(String browser) {
        URL hubUrl;
        try {
            hubUrl = new URI(grid_url).toURL();
        } catch (URISyntaxException | MalformedURLException err) {
            throw new IllegalArgumentException("Invalid grid URL");
        }

        if (browser.equalsIgnoreCase("chrome")) {
            ChromeOptions options = new ChromeOptions();
            options.addArguments("--headless");
            return new RemoteWebDriver(hubUrl, options);
        } else if (browser.equalsIgnoreCase("firefox")) {
            FirefoxOptions options = new FirefoxOptions();
            options.addArguments("-headless");
            return new RemoteWebDriver(hubUrl, options);
        } else {
            throw new IllegalArgumentException("Unsupported browser: " + browser);
        }
    }

    private static WebDriver getLocalDriver(String browser) {
        if (browser.equalsIgnoreCase("chrome")) {
            return new ChromeDriver();
        } else if (browser.equalsIgnoreCase("firefox")) {
            return new FirefoxDriver();
        } else {
            throw new IllegalArgumentException("Unsupported browser: " + browser);
        }
    }
}