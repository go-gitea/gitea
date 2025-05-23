package UITests;

import org.example.*;
import org.junit.jupiter.api.*;
import org.openqa.selenium.By;
import org.openqa.selenium.TimeoutException;
import org.openqa.selenium.WebDriver;
import org.openqa.selenium.WebElement;
import org.openqa.selenium.support.ui.ExpectedConditions;
import org.openqa.selenium.support.ui.Wait;
import org.openqa.selenium.support.ui.WebDriverWait;

import java.net.MalformedURLException;
import java.time.Duration;

import static org.example.DriverFactory.getDriver;
import static org.junit.jupiter.api.Assertions.*;

@TestMethodOrder(MethodOrderer.OrderAnnotation.class)
public class NewProjectPageTest {
    private WebDriver driver;
    private ProjectPage projectPage;
    private LoginGit login;
    private HomePageGit home;
    private ProfilePage profile;
    private NewProjectPage newProjectPage;
    private final String URL = "http://localhost:3000/user/login";
    String URL2="https://a7bd-2a06-c701-78fb-bc00-e162-b721-5502-6b4b.ngrok-free.app/user/login";

    @BeforeEach
    public void setUp() throws MalformedURLException {
        driver = getDriver();
        driver.manage().window().maximize();
        driver.get(URL);
        try {
            Wait<WebDriver> wait = new WebDriverWait(driver, Duration.ofSeconds(5));
            WebElement visitButton = wait.until(ExpectedConditions.elementToBeClickable(By.xpath("//button[text()='Visit Site']")));
            visitButton.click();
        } catch (TimeoutException err) {
            System.out.println("Ngrok warning page was not loaded");
        }
        login = new LoginGit(driver).get();
    }

    @Test
    @Order(1)
    public void testCreateProject() {
        //the bot pattern
        newProjectPage = login.loginAsValidUser("maias", "Maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test1 Project");
        newProjectPage.enterDescription("This is a test project description.");
        newProjectPage.selectTemplate("None");
        newProjectPage.selectCardPreview("Images and Text");
        newProjectPage.clickCreateProject();
        assertTrue(newProjectPage.isSuccessfulProjectPage());
    }
   @Test
   @Order(2)//if we have emty title the project will not create
    public void testEmptyFields() {
        //the bot pattern
        newProjectPage=login.loginAsValidUser("maias", "Maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("");
        newProjectPage.enterDescription("this is Description");
        newProjectPage.clickCreateProject();
        assertFalse(newProjectPage.isSuccessfulProjectPage());


    }
    @Test
    @Order(3)
    //if we have emty title the project will not create
    public void testTitleOnly() {
        //the bot pattern
        newProjectPage=login.loginAsValidUser("maias", "Maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test 3 Project2");
        newProjectPage.clickCreateProject();
        assertTrue(newProjectPage.isSuccessfulProjectPage());


    }
  /*  @Test  //if we have emty title the project will not create
    public void testCancelButton() {
        newProjectPage=login.loginAsValidUser("maias", "maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test 4 Project");
        newProjectPage.enterDescription("This is a test project description.");
        newProjectPage.selectTemplate("None");
        newProjectPage.selectCardPreview("Images and Text");
        int numberProjectBefore= newProjectPage.getNumberOfProjects();
        newProjectPage.clickCancel();
        int numberProjectafter=newProjectPage.getNumberOfProjects();
        assertEquals(numberProjectBefore,numberProjectafter);


    }

    @Test// test BoldStyle after we write the description
    public void testBoldStyleInDescription() {
        newProjectPage=login.loginAsValidUser("maias", "maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test5 Project");
        newProjectPage.enterDescription("This is a test description.");
        newProjectPage.selectTemplate("Basic Kanban");
        newProjectPage.selectCardPreview("Text Only");
        // Apply bold styling
        newProjectPage.clickBoldButton();
        // Get the HTML content after applying bold
        String afterBoldHtml = newProjectPage.getDescriptionContent();
        // Verify that bold tags are added in the HTML
        assertTrue(afterBoldHtml.contains("****") ,
                "Bold styling was not applied in the description preview.");
        //Verify the content inside the bold tags is as expected
        assertTrue(afterBoldHtml.contains("This is a test description."),
                "Description content is incorrect after applying bold styling.");
        // Create the project
        newProjectPage.clickCreateProject();
        // Verify that the project was created successfully
        assertTrue(newProjectPage.isSuccessfulProjectPage());
    }
    @Test// test BoldStyle before we write the description
    public void testBoldStyleInDescription2() {
        newProjectPage=login.loginAsValidUser("maias", "maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test6 Project");
        newProjectPage.clickBoldButton();
        newProjectPage.enterDescription("This is a test description.");
        newProjectPage.selectTemplate("Basic Kanban");
        newProjectPage.selectCardPreview("Text Only");
        // Get the HTML content after applying bold
        String afterBoldHtml = newProjectPage.getDescriptionContent();
        // Verify that bold tags are added in the HTML
        assertTrue(afterBoldHtml.equals("**This is a test description.**") ,
                "Bold styling was not applied in the description preview.");
        //Verify the content inside the bold tags is as expected
        assertTrue(afterBoldHtml.contains("This is a test description."),
                "Description content is incorrect after applying bold styling.");
        // Create the project
        newProjectPage.clickCreateProject();
        // Verify that the project was created successfully
        assertTrue(newProjectPage.isSuccessfulProjectPage());
    }
    @Test// test BoldStyle for some words we write the description
    public void testBoldStyleInDescription3() {
        newProjectPage=login.loginAsValidUser("maias", "maias123").goToProfilePage().goToProjectsPage().goToNewProjectPage();
        newProjectPage.enterTitle("Test 7 Project");
        newProjectPage.enterDescription("This is a test description.");
        newProjectPage.clickBoldButton();
        newProjectPage.enterDescription("maias ");
        newProjectPage.selectTemplate("Basic Kanban");
        newProjectPage.selectCardPreview("Text Only");
        newProjectPage.enterDescription("This not Bold");
        String afterAdditionalText=newProjectPage.getDescriptionContent();
        assertTrue(afterAdditionalText.equals("This is a test description.**maias **This not Bold"),"Bold styling was not applied in the description preview.");
        newProjectPage.clickCreateProject();
        assertTrue(newProjectPage.isSuccessfulProjectPage());
    }*/
  @AfterEach
  public void tearDown() {
    driver.quit();
  }

}

