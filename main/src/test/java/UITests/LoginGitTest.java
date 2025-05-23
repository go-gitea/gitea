package UITests;

import org.example.HomePageGit;
import org.example.LoginGit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
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
import static org.junit.jupiter.api.Assertions.assertTrue;

public class LoginGitTest {
    WebDriver driver;
    private LoginGit login;
    private final String URL="http://localhost:3000/user/login";
    //String URL="https://a7bd-2a06-c701-78fb-bc00-e162-b721-5502-6b4b.ngrok-free.app/user/login";


    @BeforeEach
    public void setUp() throws MalformedURLException {
        driver= getDriver();
        driver.manage().timeouts().implicitlyWait(Duration.ofSeconds(15));
        driver.manage().window().maximize();
        driver.get(URL);

        try{
            Wait<WebDriver> wait=new WebDriverWait(driver, Duration.ofSeconds(5));
            WebElement visitButton= wait.until(ExpectedConditions.elementToBeClickable(By.xpath("//button[text()='Visit Site']")));
            visitButton.click();
        }
        catch (TimeoutException err){
            System.out.println("Ngrok warning page was not loaded");
        }
        login = new LoginGit(driver).get();

    }
    @Test
    public void testInvalidLogin() {
        LoginGit page = login.loginWithInvalidCredentials("wrong", "wrong");
        assertTrue(page.isLoginFailed(), "Login should fail with invalid credentials");
    }

    @Test
    public void testValidLogin() {
        HomePageGit home = login.loginAsValidUser("maias", "maias123");
        assertTrue(home.isLoggedInSuccessfully(), "Login should be successful with valid credentials");
    }

    @Test
    public void testEmptyUsername() {
        LoginGit page = login.loginWithInvalidCredentials("", "Maias123");
        assertTrue(page.isLoginFailed(), "Login should fail when the username is empty");
    }

    @Test
    public void testEmptyPassword() {
        LoginGit page = login.loginWithInvalidCredentials("maias", "");
        assertTrue(page.isLoginFailed(), "Login should fail when the password is empty");
    }

    @Test
    public void testEmptyUsernameAndPassword() {
        LoginGit page = login.loginWithInvalidCredentials("", "");
        assertTrue(page.isLoginFailed(), "Login should fail when both username and password are empty");
    }
    @AfterEach
    public void tearDown() {
        driver.quit();
    }

}
