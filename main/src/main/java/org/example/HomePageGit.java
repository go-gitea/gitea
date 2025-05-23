package org.example;

import org.junit.jupiter.api.Assertions;
import org.openqa.selenium.By;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.support.PageFactory;
import org.openqa.selenium.support.ui.ExpectedConditions;
import org.openqa.selenium.support.ui.LoadableComponent;
import org.openqa.selenium.support.ui.Wait;
import org.openqa.selenium.support.ui.WebDriverWait;

import java.time.Duration;
import java.util.List;

public class HomePageGit extends LoadableComponent<HomePageGit> {
  private WebDriver driver;
  private List<WebElement> elements;
  private final String baseURL = "http://localhost:3000/user/login";
  //String baseURL2="https://a7bd-2a06-c701-78fb-bc00-e162-b721-5502-6b4b.ngrok-free.app/user/login";
  //String baseURL = System.getenv("URL");

  @Override
  protected void load() {
    this.driver.manage().timeouts().implicitlyWait(Duration.ofSeconds(10));
    driver.get(baseURL + "/");
    System.out.println(driver.getCurrentUrl());

  }

  // @Override
  protected void isLoaded() throws Error {
    Assertions.assertTrue(driver.getTitle().contains("Dashboard"));

  }

  // Constructor
  public HomePageGit(WebDriver driver) {
    this.driver = driver;
    PageFactory.initElements(driver, this);

  }

  // Method to initialize the elements list (called after the driver is initialized)
  public void initializeElements() {
    elements = driver.findElements(By.cssSelector("img.avatar, #_aria_auto_id_5")); // Adjust selectors accordingly
  }

  public boolean isLoggedInSuccessfully() {
    System.out.println(driver.getTitle());
    return driver.getTitle().contains("maias");
  }

  // Method to click on the image and then the profile button
  public ProfilePage goToProfilePage() {
    WebElement profileImage = driver.findElement(By.xpath("//*[@id=\"navbar\"]/div[2]/div[2]/span/img"));
    profileImage.click();
    WebElement profilebtn=driver.findElement(By.xpath("//*[@id=\"_aria_auto_id_5\"]"));
    profilebtn.click();
    // Initialize the elements list here before accessing the elements
      /*initializeElements();

        // Assuming the first element in the list is the profile image (img.avatar)
      WebElement profileImage = elements.get(0); // Index 0 is for the image element
      WebDriverWait wait1 = new WebDriverWait(driver, Duration.ofSeconds(30));
      WebElement dropdownButton = wait1.until(ExpectedConditions.elementToBeClickable(elements.get(1)));// Index 1 is for the profile button

        // Wait for the profile image to be clickable and then click it
        Wait<WebDriver> wait = new WebDriverWait(driver, Duration.ofSeconds(500), Duration.ofMillis(500));
        wait.until(ExpectedConditions.elementToBeClickable(profileImage)).click();

        // Wait for the profile button to be clickable and then click it
        wait.until(ExpectedConditions.elementToBeClickable(dropdownButton)).click();*/

        // Return a new ProfilePageGit object (you need to create this class to represent the profile page)
        return new ProfilePage(driver);
  }
}



