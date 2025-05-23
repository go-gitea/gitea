package org.example;

import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.support.PageFactory;
import org.openqa.selenium.support.ui.LoadableComponent;
import org.openqa.selenium.support.ui.WebDriverWait;

import java.time.Duration;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;


public class ProfilePage extends LoadableComponent<ProfilePage> {
    private WebDriver driver;
    private final String baseURL="http://localhost:3000/user/login";
    //private static final String baseURL1="https://a7bd-2a06-c701-78fb-bc00-e162-b721-5502-6b4b.ngrok-free.app/user/login";

    // Locator for the list of project links
    private By projectLinksBy = By.cssSelector("div.overflow-menu-items a[href='/maias/-/projects']");

    public ProfilePage(WebDriver driver) {
        this.driver = driver;
        // This call sets the WebElement fields.
        PageFactory.initElements(driver, this);

    }

    // Method to click on the Projects link (choosing from the list)
    public ProjectPage goToProjectsPage() {
        WebDriverWait wait = new WebDriverWait(driver, Duration.ofSeconds(10));

        // Wait until at least one project link is visible (lambda function for custom condition)
        List<WebElement> projectLinks = wait.until(driver -> driver.findElements(projectLinksBy));
        System.out.println(projectLinks);
        // Select and click on the first available project link (or you can choose any specific link)
        if (!projectLinks.isEmpty()) {
            WebElement SecondProjectLink = projectLinks.get(0); // Choose based on index (0 for the first link)
            SecondProjectLink.click();
        }

        // Return a new ProjectPage object
        return new ProjectPage(driver);
    }

    @Override
    protected void load() {
        this.driver.manage().timeouts().implicitlyWait(Duration.ofSeconds(2));
        driver.get(baseURL+"/maias");


    }

    @Override
    protected void isLoaded() throws Error {
        assertTrue(driver.getTitle().contains("maias - Gitea"));

    }
}
